package handler

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	platformlogic "appforge/admin-api/internal/logic"
	"appforge/admin-api/internal/svc"
	"appforge/common/storage"
	"appforge/common/utils"
	"appforge/proto/common"
	"appforge/proto/core"

	"github.com/zeromicro/go-zero/core/stores/redis"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	agentTransferTicketTTL = 5 * time.Minute
	agentTransferStateTTL  = 24 * time.Hour
	maxAgentAPKBytes       = int64(2 * 1024 * 1024 * 1024)
	maxAgentLogBytes       = int64(256 * 1024 * 1024)
)

type localAgentArtifactRefreshRequest struct {
	Auth           *core.LocalAgentAuth `json:"auth"`
	TaskID         int64                `json:"task_id"`
	BuilderAttempt int32                `json:"builder_attempt"`
}

type localAgentArtifactRefreshResponse struct {
	Base   *common.RespBase         `json:"base,omitempty"`
	Bundle *localAgentBuildManifest `json:"bundle"`
}

type agentArtifactTransfers struct {
	objects  storage.ObjectStore
	registry agentArtifactRegistry
	now      func() time.Time
}

type agentArtifactTicket struct {
	Kind                         string                 `json:"kind"`
	AgentID                      int64                  `json:"agent_id"`
	CertificateSerial            string                 `json:"certificate_serial"`
	CertificateFingerprintSHA256 string                 `json:"certificate_fingerprint_sha256"`
	TaskID                       int64                  `json:"task_id"`
	BuilderAttempt               int32                  `json:"builder_attempt"`
	TenantID                     int64                  `json:"tenant_id"`
	AppID                        int64                  `json:"app_id"`
	Role                         string                 `json:"role"`
	ObjectID                     int64                  `json:"object_id,omitempty"`
	ObjectType                   core.StorageObjectType `json:"object_type"`
	ObjectKey                    string                 `json:"object_key,omitempty"`
	OriginalName                 string                 `json:"original_name,omitempty"`
	ContentType                  string                 `json:"content_type,omitempty"`
	SizeBytes                    int64                  `json:"size_bytes,omitempty"`
	SHA256                       string                 `json:"sha256,omitempty"`
	ExpiresAt                    int64                  `json:"expires_at"`
}

type agentArtifactTask struct {
	AgentID                      int64  `json:"agent_id"`
	CertificateSerial            string `json:"certificate_serial"`
	CertificateFingerprintSHA256 string `json:"certificate_fingerprint_sha256"`
	TaskID                       int64  `json:"task_id"`
	BuilderAttempt               int32  `json:"builder_attempt"`
	TenantID                     int64  `json:"tenant_id"`
	AppID                        int64  `json:"app_id"`
}

type agentUploadedArtifact struct {
	Role        string                 `json:"role"`
	ObjectID    int64                  `json:"object_id"`
	ObjectKey   string                 `json:"object_key"`
	ObjectType  core.StorageObjectType `json:"object_type"`
	SizeBytes   int64                  `json:"size_bytes"`
	SHA256      string                 `json:"sha256"`
	ContentType string                 `json:"content_type"`
}

type agentArtifactUploadResponse struct {
	Reference string `json:"reference"`
	SHA256    string `json:"sha256"`
	SizeBytes int64  `json:"size_bytes"`
}

func newAgentArtifactTransfers(ctx context.Context, svcCtx *svc.ServiceContext) (*agentArtifactTransfers, error) {
	if len(svcCtx.Config.CacheRedis) == 0 {
		return nil, errors.New("Local Agent Artifact ticket Redis is not configured")
	}
	redisClient, err := redis.NewRedis(svcCtx.Config.CacheRedis[0].RedisConf)
	if err != nil {
		return nil, fmt.Errorf("connect Local Agent Artifact ticket Redis: %w", err)
	}
	objectStore, err := platformlogic.LoadObjectStore(ctx, svcCtx)
	if err != nil {
		return nil, err
	}
	return newAgentArtifactTransfersWithDependencies(objectStore, newRedisAgentArtifactRegistry(redisClient)), nil
}

func newAgentArtifactTransfersWithDependencies(objects storage.ObjectStore, registry agentArtifactRegistry) *agentArtifactTransfers {
	return &agentArtifactTransfers{objects: objects, registry: registry, now: time.Now}
}

func (a *agentArtifactTransfers) issueBundle(ctx context.Context, bundle *localAgentBuildManifest, agentID int64, identity agentTLSIdentity, refresh bool) error {
	if bundle == nil || bundle.Task == nil || bundle.Task.Id <= 0 || bundle.Task.BuilderAttempt <= 0 || agentID <= 0 {
		return status.Error(codes.FailedPrecondition, "Local Agent Artifact task identity is incomplete")
	}
	taskKey := agentTransferTaskKey(agentID, bundle.Task.Id, bundle.Task.BuilderAttempt)
	state := agentArtifactTask{
		AgentID: agentID, CertificateSerial: identity.serial, CertificateFingerprintSHA256: identity.fingerprint,
		TaskID: bundle.Task.Id, BuilderAttempt: bundle.Task.BuilderAttempt, TenantID: bundle.Task.TenantId, AppID: bundle.Task.AppId,
	}
	if existing, err := a.registry.GetTask(ctx, taskKey); err == nil {
		if err := validateAgentArtifactTask(existing, &state, identity); err != nil {
			return err
		}
	} else if refresh || !errors.Is(err, errAgentArtifactStateNotFound) {
		if errors.Is(err, errAgentArtifactStateNotFound) {
			return status.Error(codes.FailedPrecondition, "Local Agent Artifact task state expired")
		}
		return status.Errorf(codes.Internal, "load Local Agent Artifact task state: %v", err)
	}
	if err := a.registry.PutTask(ctx, taskKey, state, agentTransferStateTTL); err != nil {
		return status.Errorf(codes.Internal, "store Local Agent Artifact task state: %v", err)
	}
	expiresAt := a.now().Add(agentTransferTicketTTL)
	for index := range bundle.Inputs {
		input := &bundle.Inputs[index]
		if input.StorageMode != core.HybridArtifactMode_HYBRID_ARTIFACT_MODE_CONTROL_PLANE_STORAGE || input.OwnerAgentID != 0 {
			return status.Errorf(codes.FailedPrecondition, "Local Agent %s input is not a control-plane object", input.Role)
		}
		if strings.TrimSpace(input.objectKey) == "" {
			return status.Errorf(codes.FailedPrecondition, "Local Agent %s object key is unavailable", input.Role)
		}
		token, err := newAgentTransferToken()
		if err != nil {
			return status.Errorf(codes.Internal, "create Artifact download ticket: %v", err)
		}
		ticket := agentArtifactTicket{
			Kind: "download", AgentID: agentID, CertificateSerial: identity.serial,
			CertificateFingerprintSHA256: identity.fingerprint, TaskID: bundle.Task.Id, BuilderAttempt: bundle.Task.BuilderAttempt,
			TenantID: bundle.Task.TenantId, AppID: bundle.Task.AppId, Role: input.Role, ObjectID: input.ObjectID,
			ObjectType: input.ObjectType, ObjectKey: input.objectKey, OriginalName: input.OriginalName,
			ContentType: input.ContentType, SizeBytes: input.SizeBytes, SHA256: input.SHA256, ExpiresAt: expiresAt.Unix(),
		}
		if err := a.registry.PutTicket(ctx, token, ticket, agentTransferTicketTTL); err != nil {
			return status.Errorf(codes.Internal, "store Artifact download ticket: %v", err)
		}
		input.DownloadPath = "/v1/artifacts/download/" + token
	}
	bundle.Outputs = bundle.Outputs[:0]
	for _, output := range []struct {
		role       string
		objectType core.StorageObjectType
	}{{"built_apk", core.StorageObjectType_STORAGE_OBJECT_TYPE_BUILT_APK}, {"build_log", core.StorageObjectType_STORAGE_OBJECT_TYPE_BUILD_LOG}} {
		token, err := newAgentTransferToken()
		if err != nil {
			return status.Errorf(codes.Internal, "create Artifact upload ticket: %v", err)
		}
		ticket := agentArtifactTicket{
			Kind: "upload", AgentID: agentID, CertificateSerial: identity.serial,
			CertificateFingerprintSHA256: identity.fingerprint, TaskID: bundle.Task.Id, BuilderAttempt: bundle.Task.BuilderAttempt,
			TenantID: bundle.Task.TenantId, AppID: bundle.Task.AppId, Role: output.role, ObjectType: output.objectType,
			ExpiresAt: expiresAt.Unix(),
		}
		if err := a.registry.PutTicket(ctx, token, ticket, agentTransferTicketTTL); err != nil {
			return status.Errorf(codes.Internal, "store Artifact upload ticket: %v", err)
		}
		bundle.Outputs = append(bundle.Outputs, localAgentBuildOutput{Role: output.role, UploadPath: "/v1/artifacts/upload/" + token, ExpiresAt: expiresAt.Unix()})
	}
	return nil
}

func (a *agentArtifactTransfers) downloadHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeAgentError(w, status.Error(codes.Unimplemented, "method is not allowed"))
			return
		}
		identity, err := verifiedAgentIdentity(r)
		if err != nil {
			writeAgentError(w, err)
			return
		}
		ticket, err := a.consumeTicket(r.Context(), strings.TrimPrefix(r.URL.Path, "/v1/artifacts/download/"), "download", identity)
		if err != nil {
			writeAgentError(w, err)
			return
		}
		info, err := a.objects.StatObject(r.Context(), ticket.ObjectKey)
		if err != nil || info.Size != ticket.SizeBytes {
			writeAgentError(w, status.Error(codes.FailedPrecondition, "Artifact input object changed or is unavailable"))
			return
		}
		reader, err := a.objects.OpenObject(r.Context(), ticket.ObjectKey)
		if err != nil {
			writeAgentError(w, status.Error(codes.FailedPrecondition, "Artifact input object is unavailable"))
			return
		}
		defer reader.Close()
		w.Header().Set("Content-Type", ticket.ContentType)
		w.Header().Set("Content-Length", fmt.Sprint(ticket.SizeBytes))
		w.Header().Set("X-AppForge-Sha256", ticket.SHA256)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = io.CopyN(w, reader, ticket.SizeBytes)
	}
}

func (a *agentArtifactTransfers) uploadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			writeAgentError(w, status.Error(codes.Unimplemented, "method is not allowed"))
			return
		}
		identity, err := verifiedAgentIdentity(r)
		if err != nil {
			writeAgentError(w, err)
			return
		}
		declaredDigest := strings.ToLower(strings.TrimSpace(r.Header.Get("X-AppForge-Sha256")))
		if r.ContentLength <= 0 || !isAgentSHA256(declaredDigest) {
			writeAgentError(w, status.Error(codes.InvalidArgument, "Content-Length and X-AppForge-Sha256 are required"))
			return
		}
		ticket, err := a.consumeTicket(r.Context(), strings.TrimPrefix(r.URL.Path, "/v1/artifacts/upload/"), "upload", identity)
		if err != nil {
			writeAgentError(w, err)
			return
		}
		maxBytes := maxAgentLogBytes
		if ticket.ObjectType == core.StorageObjectType_STORAGE_OBJECT_TYPE_BUILT_APK {
			maxBytes = maxAgentAPKBytes
		}
		if r.ContentLength > maxBytes {
			writeAgentError(w, status.Error(codes.InvalidArgument, "Artifact output exceeds the allowed size"))
			return
		}
		name, contentType := agentOutputMetadata(ticket)
		rpcCtx := context.WithValue(r.Context(), utils.CtxKeyTenantId, ticket.TenantID)
		objectKey, err := platformlogic.GenerateStorageObjectKey(rpcCtx, ticket.ObjectType, name)
		if err != nil {
			writeAgentError(w, err)
			return
		}
		created, err := svcCtx.CoreCli.CreateStorageObject(rpcCtx, &core.CreateStorageObjectReq{
			AppId: ticket.AppID, ObjectType: ticket.ObjectType, ObjectKey: objectKey, OriginalName: name,
			ContentType: contentType, SizeBytes: r.ContentLength,
		})
		if err != nil {
			writeAgentError(w, err)
			return
		}
		if created.GetData() == nil {
			writeAgentError(w, status.Error(codes.Internal, "created Artifact object metadata is missing"))
			return
		}
		hasher := sha256.New()
		counter := &agentCountingReader{reader: io.TeeReader(r.Body, hasher)}
		if err := a.objects.PutObject(rpcCtx, objectKey, counter, r.ContentLength, contentType); err != nil {
			platformlogic.CleanupFailedUpload(rpcCtx, svcCtx, a.objects, created.Data)
			writeAgentError(w, status.Errorf(codes.Internal, "upload Artifact output: %v", err))
			return
		}
		actualDigest := hex.EncodeToString(hasher.Sum(nil))
		info, statErr := a.objects.StatObject(rpcCtx, objectKey)
		if statErr != nil || info.Size != r.ContentLength || counter.count != r.ContentLength ||
			subtle.ConstantTimeCompare([]byte(actualDigest), []byte(declaredDigest)) != 1 {
			platformlogic.CleanupFailedUpload(rpcCtx, svcCtx, a.objects, created.Data)
			writeAgentError(w, status.Error(codes.FailedPrecondition, "Artifact output size or SHA-256 mismatch"))
			return
		}
		upload := agentUploadedArtifact{Role: ticket.Role, ObjectID: created.Data.Id, ObjectKey: objectKey,
			ObjectType: ticket.ObjectType, SizeBytes: counter.count, SHA256: actualDigest, ContentType: contentType}
		selected, err := a.recordUpload(rpcCtx, ticket, identity, upload)
		if err != nil {
			platformlogic.CleanupFailedUpload(rpcCtx, svcCtx, a.objects, created.Data)
			writeAgentError(w, err)
			return
		}
		if selected.ObjectID != upload.ObjectID {
			platformlogic.CleanupFailedUpload(rpcCtx, svcCtx, a.objects, created.Data)
		}
		writeAgentJSON(w, &agentArtifactUploadResponse{Reference: storageObjectReference(selected.ObjectID), SHA256: selected.SHA256, SizeBytes: selected.SizeBytes}, nil)
	}
}

func (a *agentArtifactTransfers) finalizeTask(ctx context.Context, svcCtx *svc.ServiceContext, req *core.CompleteLocalAgentBuildTaskReq, identity agentTLSIdentity) error {
	if req == nil || req.Auth == nil {
		return status.Error(codes.InvalidArgument, "completion request is required")
	}
	state, err := a.optionalTaskState(ctx, req.Auth.AgentId, req.TaskId, req.BuilderAttempt, identity)
	if err != nil {
		return err
	}
	if state == nil {
		if strings.HasPrefix(strings.TrimSpace(req.ApkReference), "storage-object://") {
			return status.Error(codes.FailedPrecondition, "Local Agent Artifact task state expired; task must be recovered")
		}
		return nil
	}
	apk, err := a.registry.GetUpload(ctx, agentTransferUploadKey(req.Auth.AgentId, req.TaskId, req.BuilderAttempt, "built_apk"))
	if err != nil {
		return status.Error(codes.FailedPrecondition, "control-plane APK upload is required")
	}
	rpcCtx := context.WithValue(ctx, utils.CtxKeyTenantId, state.TenantID)
	if err := a.verifyAndCompleteUpload(rpcCtx, svcCtx, state, apk); err != nil {
		return err
	}
	req.ApkReference, req.ApkSha256, req.ApkSize = storageObjectReference(apk.ObjectID), apk.SHA256, apk.SizeBytes
	logArtifact, logErr := a.registry.GetUpload(ctx, agentTransferUploadKey(req.Auth.AgentId, req.TaskId, req.BuilderAttempt, "build_log"))
	if logErr == nil {
		if err := a.verifyAndCompleteUpload(rpcCtx, svcCtx, state, logArtifact); err != nil {
			return err
		}
		req.LogReference, req.LogSha256, req.LogSize = storageObjectReference(logArtifact.ObjectID), logArtifact.SHA256, logArtifact.SizeBytes
	} else if errors.Is(logErr, errAgentArtifactStateNotFound) {
		req.LogReference, req.LogSha256, req.LogSize = "", "", 0
	} else {
		return status.Errorf(codes.Internal, "load build log upload state: %v", logErr)
	}
	return nil
}

func (a *agentArtifactTransfers) finalizeFailure(ctx context.Context, svcCtx *svc.ServiceContext, req *core.FailLocalAgentBuildTaskReq, identity agentTLSIdentity) error {
	if req == nil || req.Auth == nil {
		return status.Error(codes.InvalidArgument, "failure request is required")
	}
	state, err := a.optionalTaskState(ctx, req.Auth.AgentId, req.TaskId, req.BuilderAttempt, identity)
	if err != nil {
		return err
	}
	if state == nil {
		if strings.HasPrefix(strings.TrimSpace(req.LogReference), "storage-object://") {
			return status.Error(codes.FailedPrecondition, "Local Agent Artifact task state expired; task must be recovered")
		}
		return nil
	}
	logArtifact, err := a.registry.GetUpload(ctx, agentTransferUploadKey(req.Auth.AgentId, req.TaskId, req.BuilderAttempt, "build_log"))
	if errors.Is(err, errAgentArtifactStateNotFound) {
		req.LogReference, req.LogSha256, req.LogSize = "", "", 0
		return nil
	}
	if err != nil {
		return status.Errorf(codes.Internal, "load build log upload state: %v", err)
	}
	rpcCtx := context.WithValue(ctx, utils.CtxKeyTenantId, state.TenantID)
	if err := a.verifyAndCompleteUpload(rpcCtx, svcCtx, state, logArtifact); err != nil {
		return err
	}
	req.LogReference, req.LogSha256, req.LogSize = storageObjectReference(logArtifact.ObjectID), logArtifact.SHA256, logArtifact.SizeBytes
	return nil
}

func (a *agentArtifactTransfers) finishTask(ctx context.Context, agentID, taskID int64, attempt int32) {
	key := agentTransferTaskKey(agentID, taskID, attempt)
	_ = a.registry.Delete(ctx, "task:"+key,
		"upload:"+agentTransferUploadKey(agentID, taskID, attempt, "built_apk"),
		"upload:"+agentTransferUploadKey(agentID, taskID, attempt, "build_log"))
}

func (a *agentArtifactTransfers) verifyAndCompleteUpload(ctx context.Context, svcCtx *svc.ServiceContext, task *agentArtifactTask, item *agentUploadedArtifact) error {
	if item == nil || task == nil || item.ObjectID <= 0 || item.SizeBytes <= 0 || !isAgentSHA256(item.SHA256) {
		return status.Error(codes.FailedPrecondition, "uploaded Artifact state is incomplete")
	}
	metadata, err := svcCtx.CoreCli.GetStorageObject(ctx, &core.StorageObjectIdReq{Id: item.ObjectID})
	if err != nil || metadata.GetData() == nil {
		return status.Error(codes.FailedPrecondition, "uploaded Artifact metadata is unavailable")
	}
	object := metadata.Data
	if object.TenantId != task.TenantID || object.AppId != task.AppID || object.ObjectKey != item.ObjectKey ||
		object.ObjectType != item.ObjectType || object.SizeBytes != item.SizeBytes {
		return status.Error(codes.PermissionDenied, "uploaded Artifact ownership or metadata mismatch")
	}
	info, err := a.objects.StatObject(ctx, item.ObjectKey)
	if err != nil || info.Size != item.SizeBytes {
		return status.Error(codes.FailedPrecondition, "uploaded Artifact changed before control-plane verification")
	}
	reader, err := a.objects.OpenObject(ctx, item.ObjectKey)
	if err != nil {
		return status.Error(codes.FailedPrecondition, "uploaded Artifact is unavailable for verification")
	}
	hasher := sha256.New()
	written, copyErr := io.Copy(hasher, io.LimitReader(reader, item.SizeBytes+1))
	closeErr := reader.Close()
	actualDigest := hex.EncodeToString(hasher.Sum(nil))
	if copyErr != nil || closeErr != nil || written != item.SizeBytes ||
		subtle.ConstantTimeCompare([]byte(actualDigest), []byte(item.SHA256)) != 1 {
		return status.Error(codes.FailedPrecondition, "uploaded Artifact failed independent size or SHA-256 verification")
	}
	switch object.Status {
	case core.StorageObjectStatus_STORAGE_OBJECT_STATUS_UPLOADING:
		_, err = svcCtx.CoreCli.CompleteStorageObject(ctx, &core.CompleteStorageObjectReq{Id: item.ObjectID, SizeBytes: written, Sha256: actualDigest})
		return err
	case core.StorageObjectStatus_STORAGE_OBJECT_STATUS_READY, core.StorageObjectStatus_STORAGE_OBJECT_STATUS_BOUND:
		if object.Sha256 != actualDigest {
			return status.Error(codes.FailedPrecondition, "completed Artifact metadata SHA-256 mismatch")
		}
		return nil
	default:
		return status.Error(codes.FailedPrecondition, "uploaded Artifact object is not completable")
	}
}

func (a *agentArtifactTransfers) consumeTicket(ctx context.Context, token, kind string, identity agentTLSIdentity) (*agentArtifactTicket, error) {
	if token == "" || strings.Contains(token, "/") {
		return nil, status.Error(codes.NotFound, "Artifact transfer ticket is invalid")
	}
	ticket, err := a.registry.ConsumeTicket(ctx, token)
	if errors.Is(err, errAgentArtifactStateNotFound) {
		return nil, status.Error(codes.NotFound, "Artifact transfer ticket is invalid, expired or already consumed")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "consume Artifact transfer ticket: %v", err)
	}
	if ticket.Kind != kind || ticket.ExpiresAt < a.now().Unix() {
		return nil, status.Error(codes.NotFound, "Artifact transfer ticket is invalid or expired")
	}
	if !ticketMatchesIdentity(ticket, identity) {
		return nil, status.Error(codes.PermissionDenied, "Artifact transfer ticket certificate mismatch")
	}
	return ticket, nil
}

func (a *agentArtifactTransfers) recordUpload(ctx context.Context, ticket *agentArtifactTicket, identity agentTLSIdentity, upload agentUploadedArtifact) (*agentUploadedArtifact, error) {
	state, err := a.requiredTaskState(ctx, ticket.AgentID, ticket.TaskID, ticket.BuilderAttempt, identity)
	if err != nil {
		return nil, err
	}
	if state.TenantID != ticket.TenantID || state.AppID != ticket.AppID {
		return nil, status.Error(codes.PermissionDenied, "Artifact upload task ownership mismatch")
	}
	key := agentTransferUploadKey(ticket.AgentID, ticket.TaskID, ticket.BuilderAttempt, ticket.Role)
	stored, err := a.registry.PutUploadIfAbsent(ctx, key, upload, agentTransferStateTTL)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "store Artifact upload state: %v", err)
	}
	if stored {
		return &upload, nil
	}
	existing, err := a.registry.GetUpload(ctx, key)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load existing Artifact upload state: %v", err)
	}
	if existing.ObjectType != upload.ObjectType || existing.SizeBytes != upload.SizeBytes || existing.SHA256 != upload.SHA256 {
		return nil, status.Error(codes.AlreadyExists, "Artifact output was already uploaded with different integrity data")
	}
	return existing, nil
}

func (a *agentArtifactTransfers) requiredTaskState(ctx context.Context, agentID, taskID int64, attempt int32, identity agentTLSIdentity) (*agentArtifactTask, error) {
	state, err := a.optionalTaskState(ctx, agentID, taskID, attempt, identity)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return nil, status.Error(codes.FailedPrecondition, "Local Agent Artifact task state expired; task must be recovered")
	}
	return state, nil
}

func (a *agentArtifactTransfers) optionalTaskState(ctx context.Context, agentID, taskID int64, attempt int32, identity agentTLSIdentity) (*agentArtifactTask, error) {
	state, err := a.registry.GetTask(ctx, agentTransferTaskKey(agentID, taskID, attempt))
	if errors.Is(err, errAgentArtifactStateNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load Local Agent Artifact task state: %v", err)
	}
	expected := &agentArtifactTask{AgentID: agentID, TaskID: taskID, BuilderAttempt: attempt,
		CertificateSerial: identity.serial, CertificateFingerprintSHA256: identity.fingerprint,
		TenantID: state.TenantID, AppID: state.AppID}
	if err := validateAgentArtifactTask(state, expected, identity); err != nil {
		return nil, err
	}
	return state, nil
}

func validateAgentArtifactTask(actual, expected *agentArtifactTask, identity agentTLSIdentity) error {
	if actual == nil || expected == nil || actual.AgentID != expected.AgentID || actual.TaskID != expected.TaskID ||
		actual.BuilderAttempt != expected.BuilderAttempt || actual.TenantID != expected.TenantID || actual.AppID != expected.AppID ||
		actual.CertificateSerial != identity.serial ||
		subtle.ConstantTimeCompare([]byte(actual.CertificateFingerprintSHA256), []byte(identity.fingerprint)) != 1 {
		return status.Error(codes.PermissionDenied, "Local Agent Artifact task ownership changed")
	}
	return nil
}

func ticketMatchesIdentity(ticket *agentArtifactTicket, identity agentTLSIdentity) bool {
	return ticket != nil && ticket.CertificateSerial == identity.serial &&
		subtle.ConstantTimeCompare([]byte(ticket.CertificateFingerprintSHA256), []byte(identity.fingerprint)) == 1
}

func newAgentTransferToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func agentTransferTaskKey(agentID, taskID int64, attempt int32) string {
	return fmt.Sprintf("%d:%d:%d", agentID, taskID, attempt)
}

func agentTransferUploadKey(agentID, taskID int64, attempt int32, role string) string {
	return fmt.Sprintf("%d:%d:%d:%s", agentID, taskID, attempt, role)
}

func isAgentSHA256(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && value == strings.ToLower(value)
}

func agentOutputMetadata(ticket *agentArtifactTicket) (string, string) {
	if ticket.ObjectType == core.StorageObjectType_STORAGE_OBJECT_TYPE_BUILT_APK {
		return fmt.Sprintf("task-%d-attempt-%d.apk", ticket.TaskID, ticket.BuilderAttempt), "application/vnd.android.package-archive"
	}
	return fmt.Sprintf("task-%d-attempt-%d.log", ticket.TaskID, ticket.BuilderAttempt), "text/plain; charset=utf-8"
}

func storageObjectReference(id int64) string { return fmt.Sprintf("storage-object://%d", id) }

type agentCountingReader struct {
	reader io.Reader
	count  int64
}

func (r *agentCountingReader) Read(buffer []byte) (int, error) {
	n, err := r.reader.Read(buffer)
	r.count += int64(n)
	return n, err
}
