package reqenc

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Middleware struct {
	service  *Service
	registry *Registry
}

func NewMiddleware(service *Service, registry *Registry) *Middleware {
	return &Middleware{service: service, registry: registry}
}

func (m *Middleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			next(w, r)
			return
		}

		rule, matched := m.registry.Match(r)
		encrypted := strings.TrimSpace(r.Header.Get(HeaderVersion)) != ""
		switch m.service.config.Mode {
		case ModeDisabled:
			if encrypted {
				writeError(w, ErrDisabled)
				return
			}
			next(w, r)
			return
		case ModeRequired:
			if !matched || rule.Exempt {
				next(w, r)
				return
			}
			if !encrypted {
				writeError(w, ErrRequired)
				return
			}
		case ModeOptional:
			if !matched || rule.Exempt || rule.RequiredOnly {
				next(w, r)
				return
			}
			if !encrypted {
				writeError(w, ErrRequired)
				return
			}
		}

		if err := m.decryptRequest(r, rule); err != nil {
			writeError(w, err)
			return
		}
		next(w, r)
	}
}

func (m *Middleware) decryptRequest(r *http.Request, rule Rule) error {
	version := strings.TrimSpace(r.Header.Get(HeaderVersion))
	location := Location(strings.ToUpper(strings.TrimSpace(r.Header.Get(HeaderLocation))))
	keyID := strings.TrimSpace(r.Header.Get(HeaderKeyID))
	timestamp := strings.TrimSpace(r.Header.Get(HeaderTimestamp))
	nonce := strings.TrimSpace(r.Header.Get(HeaderNonce))
	if version != Version || location != rule.Location || keyID == "" ||
		len(nonce) < 16 || len(nonce) > 64 || !isURLSafe(nonce) {
		return ErrInvalidPayload
	}
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || absDuration(m.service.now().Sub(time.Unix(seconds, 0))) > m.service.config.ClockSkew() {
		return ErrInvalidPayload
	}
	if location == LocationPath {
		return ErrInvalidPayload
	}

	session, err := m.service.store.GetSession(r.Context(), keyID)
	if err != nil {
		return err
	}
	now := m.service.now()
	if session.Version != Version || session.RSAKid != m.service.config.RSAKid ||
		session.ExpiresAt <= now.UnixMilli() {
		return ErrKeyExpired
	}
	fresh, err := m.service.store.UseNonce(r.Context(), keyID, nonce, m.service.config.NonceTTL())
	if err != nil {
		return err
	}
	if !fresh {
		return ErrReplayed
	}
	key, err := unwrapSessionKey([]byte(m.service.config.SessionWrapKey), session.WrappedKey)
	if err != nil {
		return ErrInvalidPayload
	}
	aad := BuildAAD(location, keyID, timestamp, nonce, r.Method, BindingTarget(r, location, rule.PathTemplate))
	plaintext, err := m.decryptPayload(r, location, key, aad)
	if err != nil || int64(len(plaintext)) > m.service.config.MaxPlaintextBytes {
		return ErrInvalidPayload
	}
	return replaceRequestPayload(r, location, plaintext)
}

func (m *Middleware) decryptPayload(r *http.Request, location Location, key []byte, aad []byte) ([]byte, error) {
	switch location {
	case LocationJSON:
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/json" {
			return nil, ErrInvalidPayload
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, m.service.config.MaxCipherTextBytes+1))
		if err != nil || int64(len(body)) > m.service.config.MaxCipherTextBytes {
			return nil, ErrInvalidPayload
		}
		var envelope JSONEnvelope
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&envelope); err != nil || envelope.IV == "" || envelope.CipherText == "" {
			return nil, ErrInvalidPayload
		}
		iv, err := base64URL.DecodeString(envelope.IV)
		if err != nil {
			return nil, ErrInvalidPayload
		}
		cipherText, err := base64URL.DecodeString(envelope.CipherText)
		if err != nil {
			return nil, ErrInvalidPayload
		}
		return decryptGCM(key, iv, cipherText, aad)
	case LocationForm:
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/x-www-form-urlencoded" {
			return nil, ErrInvalidPayload
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, m.service.config.MaxCipherTextBytes+1))
		if err != nil {
			return nil, ErrInvalidPayload
		}
		values, err := url.ParseQuery(string(body))
		if err != nil || len(values) != 1 || len(values["data"]) != 1 {
			return nil, ErrInvalidPayload
		}
		return decryptPacked(values.Get("data"), key, aad, m.service.config.MaxCipherTextBytes)
	case LocationQuery:
		values, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil || len(values) != 1 || len(values["data"]) != 1 {
			return nil, ErrInvalidPayload
		}
		return decryptPacked(values.Get("data"), key, aad, m.service.config.MaxCipherTextBytes)
	default:
		return nil, ErrInvalidPayload
	}
}

func decryptPacked(encoded string, key []byte, aad []byte, maxBytes int64) ([]byte, error) {
	raw, err := base64URL.DecodeString(encoded)
	if err != nil || int64(len(raw)) > maxBytes || len(raw) < 12+16 {
		return nil, ErrInvalidPayload
	}
	return decryptGCM(key, raw[:12], raw[12:], aad)
}

func replaceRequestPayload(r *http.Request, location Location, plaintext []byte) error {
	switch location {
	case LocationJSON, LocationForm:
		r.Body = io.NopCloser(bytes.NewReader(plaintext))
		r.ContentLength = int64(len(plaintext))
		r.Header.Set("Content-Length", strconv.Itoa(len(plaintext)))
	case LocationQuery:
		if _, err := url.ParseQuery(string(plaintext)); err != nil {
			return ErrInvalidPayload
		}
		r.URL.RawQuery = string(plaintext)
		r.Form, r.PostForm = nil, nil
	default:
		return ErrInvalidPayload
	}
	r.Header.Del(HeaderVersion)
	r.Header.Del(HeaderLocation)
	r.Header.Del(HeaderKeyID)
	r.Header.Del(HeaderTimestamp)
	r.Header.Del(HeaderNonce)
	return nil
}

func isURLSafe(value string) bool {
	for _, char := range value {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '-' || char == '_') {
			return false
		}
	}
	return true
}

func absDuration(value time.Duration) time.Duration {
	if value < 0 {
		return -value
	}
	return value
}

func writeError(w http.ResponseWriter, err error) {
	code := http.StatusBadRequest
	if errors.Is(err, ErrRequired) {
		code = http.StatusPreconditionRequired
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(Response{Code: code, Msg: err.Error()})
}
