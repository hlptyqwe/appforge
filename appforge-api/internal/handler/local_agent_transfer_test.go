package handler

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestAgentArtifactTicketIsSingleUseBoundAndExpiring(t *testing.T) {
	registry := newMemoryAgentArtifactRegistry()
	transfers := newAgentArtifactTransfersWithDependencies(nil, registry)
	baseTime := time.Unix(1_800_000_000, 0)
	transfers.now = func() time.Time { return baseTime }
	identity := agentTLSIdentity{serial: "abc", fingerprint: strings.Repeat("a", 64)}
	bundle, err := buildLocalAgentManifest(localAgentExecutionFixture())
	if err != nil {
		t.Fatal(err)
	}
	if err := transfers.issueBundle(context.Background(), bundle, 51, identity, false); err != nil {
		t.Fatal(err)
	}
	if len(bundle.Inputs) != 2 || len(bundle.Outputs) != 2 {
		t.Fatalf("unexpected transfer bundle: %#v", bundle)
	}
	firstToken := strings.TrimPrefix(bundle.Inputs[0].DownloadPath, "/v1/artifacts/download/")
	wrongIdentity := agentTLSIdentity{serial: identity.serial, fingerprint: strings.Repeat("b", 64)}
	if _, err := transfers.consumeTicket(context.Background(), firstToken, "download", wrongIdentity); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("certificate mismatch code=%v err=%v", status.Code(err), err)
	}
	if _, err := transfers.consumeTicket(context.Background(), firstToken, "download", identity); status.Code(err) != codes.NotFound {
		t.Fatalf("mismatched use did not consume ticket: code=%v err=%v", status.Code(err), err)
	}

	if err := transfers.issueBundle(context.Background(), bundle, 51, identity, true); err != nil {
		t.Fatal(err)
	}
	secondToken := strings.TrimPrefix(bundle.Inputs[0].DownloadPath, "/v1/artifacts/download/")
	if _, err := transfers.consumeTicket(context.Background(), secondToken, "download", identity); err != nil {
		t.Fatal(err)
	}
	if _, err := transfers.consumeTicket(context.Background(), secondToken, "download", identity); status.Code(err) != codes.NotFound {
		t.Fatalf("replayed ticket code=%v err=%v", status.Code(err), err)
	}

	if err := transfers.issueBundle(context.Background(), bundle, 51, identity, true); err != nil {
		t.Fatal(err)
	}
	expiredToken := strings.TrimPrefix(bundle.Inputs[0].DownloadPath, "/v1/artifacts/download/")
	transfers.now = func() time.Time { return baseTime.Add(agentTransferTicketTTL + time.Second) }
	if _, err := transfers.consumeTicket(context.Background(), expiredToken, "download", identity); status.Code(err) != codes.NotFound {
		t.Fatalf("expired ticket code=%v err=%v", status.Code(err), err)
	}
}

func TestRedisArtifactTicketKeyDoesNotContainBearerToken(t *testing.T) {
	registry := newRedisAgentArtifactRegistry(nil)
	token := "top-secret-transfer-token"
	key := registry.ticketKey(token)
	if strings.Contains(key, token) || !strings.HasPrefix(key, registry.prefix+"ticket:") {
		t.Fatalf("unsafe ticket key %q", key)
	}
}

func TestAgentArtifactUploadIsIdempotentByTaskAttemptAndRole(t *testing.T) {
	registry := newMemoryAgentArtifactRegistry()
	transfers := newAgentArtifactTransfersWithDependencies(nil, registry)
	identity := agentTLSIdentity{serial: "abc", fingerprint: strings.Repeat("c", 64)}
	bundle, err := buildLocalAgentManifest(localAgentExecutionFixture())
	if err != nil {
		t.Fatal(err)
	}
	if err := transfers.issueBundle(context.Background(), bundle, 61, identity, false); err != nil {
		t.Fatal(err)
	}
	ticket := &agentArtifactTicket{AgentID: 61, TaskID: bundle.Task.Id, BuilderAttempt: bundle.Task.BuilderAttempt,
		TenantID: bundle.Task.TenantId, AppID: bundle.Task.AppId, Role: "built_apk"}
	first := agentUploadedArtifact{Role: "built_apk", ObjectID: 101, ObjectType: 3, SizeBytes: 128, SHA256: strings.Repeat("d", 64)}
	selected, err := transfers.recordUpload(context.Background(), ticket, identity, first)
	if err != nil || selected.ObjectID != first.ObjectID {
		t.Fatalf("first upload selected=%#v err=%v", selected, err)
	}
	retry := first
	retry.ObjectID = 102
	selected, err = transfers.recordUpload(context.Background(), ticket, identity, retry)
	if err != nil || selected.ObjectID != first.ObjectID {
		t.Fatalf("idempotent retry selected=%#v err=%v", selected, err)
	}
	tampered := retry
	tampered.SHA256 = strings.Repeat("e", 64)
	if _, err := transfers.recordUpload(context.Background(), ticket, identity, tampered); status.Code(err) != codes.AlreadyExists {
		t.Fatalf("tampered retry code=%v err=%v", status.Code(err), err)
	}
}

type memoryAgentArtifactRegistry struct {
	mu      sync.Mutex
	tickets map[string]agentArtifactTicket
	tasks   map[string]agentArtifactTask
	uploads map[string]agentUploadedArtifact
}

func newMemoryAgentArtifactRegistry() *memoryAgentArtifactRegistry {
	return &memoryAgentArtifactRegistry{tickets: map[string]agentArtifactTicket{}, tasks: map[string]agentArtifactTask{}, uploads: map[string]agentUploadedArtifact{}}
}

func (m *memoryAgentArtifactRegistry) PutTicket(_ context.Context, token string, ticket agentArtifactTicket, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tickets[token] = ticket
	return nil
}

func (m *memoryAgentArtifactRegistry) ConsumeTicket(_ context.Context, token string) (*agentArtifactTicket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ticket, ok := m.tickets[token]
	if !ok {
		return nil, errAgentArtifactStateNotFound
	}
	delete(m.tickets, token)
	return &ticket, nil
}

func (m *memoryAgentArtifactRegistry) PutTask(_ context.Context, key string, task agentArtifactTask, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tasks[key] = task
	return nil
}

func (m *memoryAgentArtifactRegistry) GetTask(_ context.Context, key string) (*agentArtifactTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	task, ok := m.tasks[key]
	if !ok {
		return nil, errAgentArtifactStateNotFound
	}
	return &task, nil
}

func (m *memoryAgentArtifactRegistry) PutUploadIfAbsent(_ context.Context, key string, upload agentUploadedArtifact, _ time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.uploads[key]; ok {
		return false, nil
	}
	m.uploads[key] = upload
	return true, nil
}

func (m *memoryAgentArtifactRegistry) GetUpload(_ context.Context, key string) (*agentUploadedArtifact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	upload, ok := m.uploads[key]
	if !ok {
		return nil, errAgentArtifactStateNotFound
	}
	return &upload, nil
}

func (m *memoryAgentArtifactRegistry) Delete(_ context.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, key := range keys {
		switch {
		case strings.HasPrefix(key, "task:"):
			delete(m.tasks, strings.TrimPrefix(key, "task:"))
		case strings.HasPrefix(key, "upload:"):
			delete(m.uploads, strings.TrimPrefix(key, "upload:"))
		default:
			return errors.New("unknown memory Artifact key")
		}
	}
	return nil
}
