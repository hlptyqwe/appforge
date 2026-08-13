package reqenc

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"
)

type memoryStore struct {
	mu       sync.Mutex
	sessions map[string]Session
	nonces   map[string]bool
}

func newMemoryStore() *memoryStore {
	return &memoryStore{sessions: map[string]Session{}, nonces: map[string]bool{}}
}

func (s *memoryStore) PutSession(_ context.Context, id string, session Session, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[id] = session
	return nil
}

func (s *memoryStore) GetSession(_ context.Context, id string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[id]
	if !ok {
		return nil, ErrKeyExpired
	}
	return &session, nil
}

func (s *memoryStore) UseNonce(_ context.Context, id, nonce string, _ time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := id + ":" + nonce
	if s.nonces[key] {
		return false, nil
	}
	s.nonces[key] = true
	return true, nil
}

func (s *memoryStore) DeleteSession(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
	return nil
}

func TestJSONMiddlewareDecryptsAndRejectsReplay(t *testing.T) {
	now := time.Unix(1784199040, 0)
	store := newMemoryStore()
	wrapKey := []byte("0123456789abcdef0123456789abcdef")
	sessionKey := []byte("abcdef0123456789abcdef0123456789")
	wrapped, err := wrapSessionKey(wrapKey, sessionKey)
	if err != nil {
		t.Fatal(err)
	}
	store.sessions["key-1"] = Session{
		Version: Version, RSAKid: "rsa-1", WrappedKey: wrapped,
		ExpiresAt: now.Add(10 * time.Minute).UnixMilli(),
	}
	service := &Service{
		config: Config{
			Scope: "test", Mode: ModeRequired, RSAKid: "rsa-1",
			SessionWrapKey: string(wrapKey), ClockSkewSeconds: 120,
			NonceTTLSeconds: 120, MaxPlaintextBytes: 1024, MaxCipherTextBytes: 4096,
		}.WithDefaults(),
		store: store,
		now:   func() time.Time { return now },
	}
	middleware := NewMiddleware(service, NewRegistry(Rule{
		Method: http.MethodPost, Path: "/api/login", Location: LocationJSON,
	}))
	var received string
	handler := middleware.Handle(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received = string(body)
		w.WriteHeader(http.StatusNoContent)
	})

	timestamp := strconv.FormatInt(now.Unix(), 10)
	nonce := "0123456789abcdef"
	aad := BuildAAD(LocationJSON, "key-1", timestamp, nonce, http.MethodPost, "/api/login")
	iv, cipherText, err := encryptGCM(sessionKey, []byte(`{"username":"guest"}`), aad)
	if err != nil {
		t.Fatal(err)
	}
	envelope, _ := json.Marshal(JSONEnvelope{
		IV: base64URL.EncodeToString(iv), CipherText: base64URL.EncodeToString(cipherText),
	})
	newRequest := func() *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(envelope))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(HeaderVersion, Version)
		req.Header.Set(HeaderLocation, string(LocationJSON))
		req.Header.Set(HeaderKeyID, "key-1")
		req.Header.Set(HeaderTimestamp, timestamp)
		req.Header.Set(HeaderNonce, nonce)
		return req
	}

	first := httptest.NewRecorder()
	handler(first, newRequest())
	if first.Code != http.StatusNoContent || received != `{"username":"guest"}` {
		t.Fatalf("first request status=%d body=%q received=%q", first.Code, first.Body.String(), received)
	}

	replay := httptest.NewRecorder()
	handler(replay, newRequest())
	if replay.Code != http.StatusBadRequest {
		t.Fatalf("replay status=%d, want %d", replay.Code, http.StatusBadRequest)
	}
}

func TestRequiredModeRejectsPlaintext(t *testing.T) {
	service := &Service{config: Config{Scope: "test", Mode: ModeRequired}.WithDefaults()}
	handler := NewMiddleware(service, NewRegistry(Rule{
		Method: http.MethodPost, Path: "/api/login", Location: LocationJSON,
	})).Handle(func(http.ResponseWriter, *http.Request) {
		t.Fatal("plaintext request reached handler")
	})
	recorder := httptest.NewRecorder()
	handler(recorder, httptest.NewRequest(http.MethodPost, "/api/login", nil))
	if recorder.Code != http.StatusPreconditionRequired {
		t.Fatalf("status=%d", recorder.Code)
	}
}

func TestOptionalModeRequiresEncryptionOnlyForSelectedRoutes(t *testing.T) {
	service := &Service{config: Config{Scope: "test", Mode: ModeOptional}.WithDefaults()}
	called := 0
	handler := NewMiddleware(service, NewRegistry(
		Rule{Method: http.MethodPost, Path: "/api/selected", Location: LocationJSON},
		Rule{Method: http.MethodPost, Path: "/api/", PathPrefix: true, Location: LocationJSON, RequiredOnly: true},
	)).Handle(func(http.ResponseWriter, *http.Request) {
		called++
	})

	selected := httptest.NewRecorder()
	handler(selected, httptest.NewRequest(http.MethodPost, "/api/selected", nil))
	if selected.Code != http.StatusPreconditionRequired {
		t.Fatalf("selected route status=%d", selected.Code)
	}

	other := httptest.NewRecorder()
	handler(other, httptest.NewRequest(http.MethodPost, "/api/other", nil))
	if other.Code != http.StatusOK || called != 1 {
		t.Fatalf("unselected route status=%d called=%d", other.Code, called)
	}
}

func TestRequiredModeUsesRequiredOnlyRulesAndHonorsExemptions(t *testing.T) {
	service := &Service{config: Config{Scope: "test", Mode: ModeRequired}.WithDefaults()}
	called := 0
	handler := NewMiddleware(service, NewRegistry(
		Rule{Method: http.MethodGet, Path: "/api/bootstrap", Exempt: true},
		Rule{Method: http.MethodGet, Path: "/api/", PathPrefix: true, Location: LocationQuery, RequiredOnly: true},
	)).Handle(func(http.ResponseWriter, *http.Request) {
		called++
	})

	protected := httptest.NewRecorder()
	handler(protected, httptest.NewRequest(http.MethodGet, "/api/users", nil))
	if protected.Code != http.StatusPreconditionRequired {
		t.Fatalf("required-only route status=%d", protected.Code)
	}

	exempt := httptest.NewRecorder()
	handler(exempt, httptest.NewRequest(http.MethodGet, "/api/bootstrap", nil))
	if exempt.Code != http.StatusOK || called != 1 {
		t.Fatalf("exempt route status=%d called=%d", exempt.Code, called)
	}
}

func TestRegistryMatchesPathPrefix(t *testing.T) {
	registry := NewRegistry(Rule{
		Method: http.MethodPost, Path: "/admin/system/", PathPrefix: true, Location: LocationJSON,
	})

	request := httptest.NewRequest(http.MethodPost, "/admin/system/users/123/status", nil)
	rule, ok := registry.Match(request)
	if !ok || rule.Location != LocationJSON {
		t.Fatalf("prefix route was not matched: ok=%v rule=%+v", ok, rule)
	}

	request = httptest.NewRequest(http.MethodGet, "/admin/system/users", nil)
	if _, ok := registry.Match(request); ok {
		t.Fatal("prefix route matched a different HTTP method")
	}
}
