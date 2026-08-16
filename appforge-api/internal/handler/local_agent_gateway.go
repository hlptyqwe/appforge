package handler

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"appforge/admin-api/internal/config"
	"appforge/admin-api/internal/svc"
	"appforge/proto/common"
	"appforge/proto/core"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RegisterLocalAgentPublicHandler registers an outbound-only Agent using a
// one-time token and a CSR. The private key is generated and retained by Agent.
func RegisterLocalAgentPublicHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeAgentError(w, status.Error(codes.Unimplemented, "method is not allowed"))
			return
		}
		var req core.RegisterLocalAgentReq
		if err := decodeAgentJSON(r, &req); err != nil {
			writeAgentError(w, status.Error(codes.InvalidArgument, err.Error()))
			return
		}
		req.SourceIp = clientIP(r)
		resp, err := svcCtx.CoreCli.RegisterLocalAgent(r.Context(), &req)
		writeAgentJSON(w, resp, err)
	}
}

// StartLocalAgentGateway starts the dedicated mTLS listener. It has no command
// execution endpoint: only the fixed build lifecycle methods below are routed.
func StartLocalAgentGateway(ctx context.Context, cfg config.LocalAgentGatewayConfig, svcCtx *svc.ServiceContext) error {
	if !cfg.Enabled {
		return nil
	}
	certificate, err := tls.LoadX509KeyPair(cfg.ServerCertificate, cfg.ServerPrivateKey)
	if err != nil {
		return err
	}
	caPEM, err := os.ReadFile(cfg.ClientCACertificate)
	if err != nil {
		return err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return errors.New("Local Agent client CA is invalid")
	}
	transfers, err := newAgentArtifactTransfers(ctx, svcCtx)
	if err != nil {
		return fmt.Errorf("initialize Local Agent Artifact transfer: %w", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/heartbeat", agentRPC(svcCtx, func(ctx context.Context, body []byte, identity agentTLSIdentity) (any, error) {
		var req core.HeartbeatLocalAgentReq
		if err := decodeAgentRPCBody(body, &req); err != nil {
			return nil, err
		}
		injectAgentIdentity(req.Auth, identity)
		return svcCtx.CoreCli.HeartbeatLocalAgent(ctx, &req)
	}))
	mux.HandleFunc("/v1/claim", agentRPC(svcCtx, func(ctx context.Context, body []byte, identity agentTLSIdentity) (any, error) {
		var req core.ClaimLocalAgentBuildTaskReq
		if err := decodeAgentRPCBody(body, &req); err != nil {
			return nil, err
		}
		injectAgentIdentity(req.Auth, identity)
		claimed, err := svcCtx.CoreCli.ClaimLocalAgentBuildTask(ctx, &req)
		if err != nil || claimed.GetTask() == nil {
			return claimed, err
		}
		execution, err := svcCtx.CoreCli.GetBuildExecutionContext(ctx, &core.GetBuildExecutionContextReq{
			TaskId: claimed.Task.Id, BuilderId: fmt.Sprintf("local-%d", req.Auth.GetAgentId()),
			BuilderAttempt: claimed.Task.BuilderAttempt,
		})
		if err != nil || execution.GetData() == nil {
			if err == nil {
				err = status.Error(codes.FailedPrecondition, "Local Agent build context is unavailable")
			}
			return nil, err
		}
		bundle, err := buildLocalAgentManifest(execution.Data)
		if err != nil {
			return nil, err
		}
		if claimed.ArtifactMode == core.HybridArtifactMode_HYBRID_ARTIFACT_MODE_CONTROL_PLANE_STORAGE {
			if err := validateControlPlaneBundleInputs(bundle); err != nil {
				return nil, err
			}
			if err := transfers.issueBundle(ctx, bundle, req.Auth.GetAgentId(), identity, false); err != nil {
				return nil, err
			}
		} else if claimed.ArtifactMode == core.HybridArtifactMode_HYBRID_ARTIFACT_MODE_CUSTOMER_STORAGE {
			if err := prepareCustomerStorageBundle(bundle, claimed.CustomerStorageRef, req.Auth.GetAgentId()); err != nil {
				return nil, err
			}
		} else {
			return nil, status.Error(codes.FailedPrecondition, "Local Agent Artifact storage mode is unsupported")
		}
		return &localAgentClaimEnvelope{Base: claimed.Base, Task: claimed.Task, ArtifactMode: claimed.ArtifactMode,
			CustomerStorageRef: claimed.CustomerStorageRef, Bundle: bundle}, nil
	}))
	mux.HandleFunc("/v1/customer-storage/inputs", agentRPC(svcCtx, func(ctx context.Context, body []byte, identity agentTLSIdentity) (any, error) {
		var req core.RegisterCustomerStorageInputReq
		if err := decodeAgentRPCBody(body, &req); err != nil {
			return nil, err
		}
		injectAgentIdentity(req.Auth, identity)
		return svcCtx.CoreCli.RegisterCustomerStorageInput(ctx, &req)
	}))
	mux.HandleFunc("/v1/artifacts/refresh", agentRPC(svcCtx, func(ctx context.Context, body []byte, identity agentTLSIdentity) (any, error) {
		var req localAgentArtifactRefreshRequest
		if err := decodeAgentJSONBytes(body, &req); err != nil {
			return nil, status.Error(codes.InvalidArgument, "request body is invalid")
		}
		injectAgentIdentity(req.Auth, identity)
		if req.TaskID <= 0 || req.BuilderAttempt <= 0 {
			return nil, status.Error(codes.InvalidArgument, "task_id and builder_attempt are required")
		}
		if _, err := svcCtx.CoreCli.RenewLocalAgentTaskLease(ctx, &core.RenewLocalAgentTaskLeaseReq{
			Auth: req.Auth, TaskId: req.TaskID, BuilderAttempt: req.BuilderAttempt, LeaseSeconds: 120,
		}); err != nil {
			return nil, err
		}
		execution, err := svcCtx.CoreCli.GetBuildExecutionContext(ctx, &core.GetBuildExecutionContextReq{
			TaskId: req.TaskID, BuilderId: fmt.Sprintf("local-%d", req.Auth.GetAgentId()), BuilderAttempt: req.BuilderAttempt,
		})
		if err != nil || execution.GetData() == nil {
			if err == nil {
				err = status.Error(codes.FailedPrecondition, "Local Agent build context is unavailable")
			}
			return nil, err
		}
		bundle, err := buildLocalAgentManifest(execution.Data)
		if err != nil {
			return nil, err
		}
		if err := transfers.issueBundle(ctx, bundle, req.Auth.GetAgentId(), identity, true); err != nil {
			return nil, err
		}
		return &localAgentArtifactRefreshResponse{Base: workerAgentBase(), Bundle: bundle}, nil
	}))
	mux.HandleFunc("/v1/artifacts/download/", transfers.downloadHandler())
	mux.HandleFunc("/v1/artifacts/upload/", transfers.uploadHandler(svcCtx))
	mux.HandleFunc("/v1/tasks/renew", agentRPC(svcCtx, func(ctx context.Context, body []byte, identity agentTLSIdentity) (any, error) {
		var req core.RenewLocalAgentTaskLeaseReq
		if err := decodeAgentRPCBody(body, &req); err != nil {
			return nil, err
		}
		injectAgentIdentity(req.Auth, identity)
		return svcCtx.CoreCli.RenewLocalAgentTaskLease(ctx, &req)
	}))
	mux.HandleFunc("/v1/tasks/progress", agentRPC(svcCtx, func(ctx context.Context, body []byte, identity agentTLSIdentity) (any, error) {
		var req core.ReportLocalAgentBuildProgressReq
		if err := decodeAgentRPCBody(body, &req); err != nil {
			return nil, err
		}
		injectAgentIdentity(req.Auth, identity)
		return svcCtx.CoreCli.ReportLocalAgentBuildProgress(ctx, &req)
	}))
	mux.HandleFunc("/v1/tasks/complete", agentRPC(svcCtx, func(ctx context.Context, body []byte, identity agentTLSIdentity) (any, error) {
		var req core.CompleteLocalAgentBuildTaskReq
		if err := decodeAgentRPCBody(body, &req); err != nil {
			return nil, err
		}
		injectAgentIdentity(req.Auth, identity)
		if err := transfers.finalizeTask(ctx, svcCtx, &req, identity); err != nil {
			return nil, err
		}
		response, err := svcCtx.CoreCli.CompleteLocalAgentBuildTask(ctx, &req)
		if err == nil {
			transfers.finishTask(ctx, req.Auth.GetAgentId(), req.TaskId, req.BuilderAttempt)
		}
		return response, err
	}))
	mux.HandleFunc("/v1/tasks/fail", agentRPC(svcCtx, func(ctx context.Context, body []byte, identity agentTLSIdentity) (any, error) {
		var req core.FailLocalAgentBuildTaskReq
		if err := decodeAgentRPCBody(body, &req); err != nil {
			return nil, err
		}
		injectAgentIdentity(req.Auth, identity)
		if err := transfers.finalizeFailure(ctx, svcCtx, &req, identity); err != nil {
			return nil, err
		}
		response, err := svcCtx.CoreCli.FailLocalAgentBuildTask(ctx, &req)
		if err == nil {
			transfers.finishTask(ctx, req.Auth.GetAgentId(), req.TaskId, req.BuilderAttempt)
		}
		return response, err
	}))
	mux.HandleFunc("/v1/artifacts/verify", agentRPC(svcCtx, func(ctx context.Context, body []byte, identity agentTLSIdentity) (any, error) {
		var req core.VerifyHybridArtifactReq
		if err := decodeAgentRPCBody(body, &req); err != nil {
			return nil, err
		}
		injectAgentIdentity(req.Auth, identity)
		return svcCtx.CoreCli.VerifyHybridArtifact(ctx, &req)
	}))
	mux.HandleFunc("/v1/certificates/rotate", agentRPC(svcCtx, func(ctx context.Context, body []byte, identity agentTLSIdentity) (any, error) {
		var req core.RotateLocalAgentCertificateReq
		if err := decodeAgentRPCBody(body, &req); err != nil {
			return nil, err
		}
		injectAgentIdentity(req.Auth, identity)
		return svcCtx.CoreCli.RotateLocalAgentCertificate(ctx, &req)
	}))
	server := &http.Server{Addr: cfg.ListenOn, Handler: mux, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 30 * time.Minute, WriteTimeout: 30 * time.Minute, IdleTimeout: 90 * time.Second,
		TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate},
			ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pool}}
	listener, err := net.Listen("tcp", cfg.ListenOn)
	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	go func() { _ = server.ServeTLS(listener, "", "") }()
	return nil
}

type agentTLSIdentity struct{ serial, fingerprint string }
type agentRPCFunc func(context.Context, []byte, agentTLSIdentity) (any, error)

// localAgentClaimEnvelope is the protocol-3 response exposed by the mTLS
// Gateway. The Core BuildExecutionContext is deliberately not serialized: it
// contains control-plane ciphertext that must never be sent to an Agent.
type localAgentClaimEnvelope struct {
	Base               *common.RespBase         `json:"base,omitempty"`
	Task               *core.BuildTask          `json:"task,omitempty"`
	ArtifactMode       core.HybridArtifactMode  `json:"artifact_mode"`
	CustomerStorageRef string                   `json:"customer_storage_ref,omitempty"`
	Bundle             *localAgentBuildManifest `json:"bundle,omitempty"`
}

type localAgentBuildManifest struct {
	SchemaVersion           int32                   `json:"schema_version"`
	Task                    *core.BuildTask         `json:"task"`
	PackageName             string                  `json:"package_name"`
	APIHost                 string                  `json:"api_host"`
	ChannelName             string                  `json:"channel_name"`
	LandingURL              string                  `json:"landing_url"`
	KeyAlias                string                  `json:"key_alias"`
	SigningSecretRef        string                  `json:"signing_secret_ref,omitempty"`
	SignerCertificateSHA256 string                  `json:"signer_certificate_sha256"`
	BrandingSnapshotJSON    string                  `json:"branding_snapshot_json,omitempty"`
	TemplateSnapshotJSON    string                  `json:"template_snapshot_json,omitempty"`
	Inputs                  []localAgentBuildInput  `json:"inputs"`
	Outputs                 []localAgentBuildOutput `json:"outputs,omitempty"`
	BlockedReason           string                  `json:"blocked_reason,omitempty"`
}

type localAgentBuildInput struct {
	Role              string                  `json:"role"`
	ObjectID          int64                   `json:"object_id"`
	ObjectType        core.StorageObjectType  `json:"object_type"`
	OriginalName      string                  `json:"original_name"`
	ContentType       string                  `json:"content_type"`
	SizeBytes         int64                   `json:"size_bytes"`
	SHA256            string                  `json:"sha256"`
	StorageMode       core.HybridArtifactMode `json:"storage_mode"`
	OwnerAgentID      int64                   `json:"owner_agent_id,omitempty"`
	DownloadPath      string                  `json:"download_path,omitempty"`
	CustomerReference string                  `json:"customer_reference,omitempty"`
	objectKey         string
}

type localAgentBuildOutput struct {
	Role       string `json:"role"`
	UploadPath string `json:"upload_path"`
	ExpiresAt  int64  `json:"expires_at"`
}

func buildLocalAgentManifest(execution *core.BuildExecutionContext) (*localAgentBuildManifest, error) {
	if execution == nil || execution.GetTask() == nil || execution.GetTask().GetId() <= 0 {
		return nil, status.Error(codes.FailedPrecondition, "Local Agent build task is missing")
	}
	task := execution.GetTask()
	bundle := &localAgentBuildManifest{
		SchemaVersion: 3, Task: task, PackageName: execution.GetPackageName(), APIHost: execution.GetApiHost(),
		ChannelName: execution.GetChannelName(), LandingURL: execution.GetLandingUrl(), KeyAlias: execution.GetKeyAlias(),
		SigningSecretRef: execution.GetSecretRef(), SignerCertificateSHA256: execution.GetSignerCertificateSha256(),
		BrandingSnapshotJSON: execution.GetBrandingSnapshotJson(), TemplateSnapshotJSON: execution.GetTemplateSnapshotJson(),
		Inputs: make([]localAgentBuildInput, 0, 4+len(execution.GetTemplateFiles())),
	}
	if strings.TrimSpace(bundle.SigningSecretRef) == "" {
		bundle.BlockedReason = "LOCAL_SIGNING_SECRET_REQUIRED"
	} else if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(bundle.SigningSecretRef)), "local-file://") {
		bundle.BlockedReason = "LOCAL_SIGNING_SECRET_PROVIDER_UNSUPPORTED"
	}
	// V3 sensitive parameters are encrypted with the control-plane master key.
	// Do not send their ciphertext to Agent. A future local parameter provider
	// must replace them with customer-side references before this gate is lifted.
	if strings.Contains(bundle.TemplateSnapshotJSON, "sb1.") {
		bundle.TemplateSnapshotJSON = ""
		if bundle.BlockedReason == "" {
			bundle.BlockedReason = "LOCAL_TEMPLATE_SECRET_REQUIRED"
		}
	}
	mandatory := []struct {
		role   string
		object *core.StorageObject
	}{{"source_apk", execution.GetSourceApk()}, {"keystore", execution.GetKeystore()}}
	for _, item := range mandatory {
		if err := appendLocalAgentBuildInput(bundle, task, item.role, item.object, true); err != nil {
			return nil, err
		}
	}
	if err := appendLocalAgentBuildInput(bundle, task, "brand_logo", execution.GetBrandLogo(), false); err != nil {
		return nil, err
	}
	if err := appendLocalAgentBuildInput(bundle, task, "brand_splash", execution.GetBrandSplash(), false); err != nil {
		return nil, err
	}
	for _, object := range execution.GetTemplateFiles() {
		if err := appendLocalAgentBuildInput(bundle, task, "template_file", object, true); err != nil {
			return nil, err
		}
	}
	return bundle, nil
}

func appendLocalAgentBuildInput(bundle *localAgentBuildManifest, task *core.BuildTask, role string, object *core.StorageObject, required bool) error {
	if object == nil || object.GetId() <= 0 {
		if required {
			return status.Errorf(codes.FailedPrecondition, "Local Agent %s input is unavailable", role)
		}
		return nil
	}
	if object.GetTenantId() != task.GetTenantId() || object.GetAppId() != task.GetAppId() {
		return status.Errorf(codes.PermissionDenied, "Local Agent %s input ownership mismatch", role)
	}
	if object.GetStatus() != core.StorageObjectStatus_STORAGE_OBJECT_STATUS_READY &&
		object.GetStatus() != core.StorageObjectStatus_STORAGE_OBJECT_STATUS_BOUND {
		return status.Errorf(codes.FailedPrecondition, "Local Agent %s input is not ready", role)
	}
	digest := strings.ToLower(strings.TrimSpace(object.GetSha256()))
	if object.GetSizeBytes() <= 0 || len(digest) != sha256.Size*2 {
		return status.Errorf(codes.FailedPrecondition, "Local Agent %s input integrity metadata is incomplete", role)
	}
	if _, err := hex.DecodeString(digest); err != nil {
		return status.Errorf(codes.FailedPrecondition, "Local Agent %s input SHA-256 is invalid", role)
	}
	bundle.Inputs = append(bundle.Inputs, localAgentBuildInput{
		Role: role, ObjectID: object.GetId(), ObjectType: object.GetObjectType(), OriginalName: object.GetOriginalName(),
		ContentType: object.GetContentType(), SizeBytes: object.GetSizeBytes(), SHA256: digest,
		StorageMode: object.GetStorageMode(), OwnerAgentID: object.GetOwnerAgentId(), objectKey: object.GetObjectKey(),
	})
	return nil
}

func validateControlPlaneBundleInputs(bundle *localAgentBuildManifest) error {
	if bundle == nil {
		return status.Error(codes.FailedPrecondition, "control-plane storage bundle is unavailable")
	}
	for _, input := range bundle.Inputs {
		if input.StorageMode != core.HybridArtifactMode_HYBRID_ARTIFACT_MODE_CONTROL_PLANE_STORAGE || input.OwnerAgentID != 0 {
			return status.Errorf(codes.FailedPrecondition, "Local Agent %s input storage ownership mismatch", input.Role)
		}
	}
	return nil
}

func prepareCustomerStorageBundle(bundle *localAgentBuildManifest, storageReference string, agentID int64) error {
	if bundle == nil || bundle.Task == nil || agentID <= 0 {
		return status.Error(codes.FailedPrecondition, "customer storage task identity is incomplete")
	}
	prefix, err := customerStoragePrefix(storageReference)
	if err != nil {
		return err
	}
	for index := range bundle.Inputs {
		input := &bundle.Inputs[index]
		key := strings.TrimSpace(input.objectKey)
		if input.StorageMode != core.HybridArtifactMode_HYBRID_ARTIFACT_MODE_CUSTOMER_STORAGE || input.OwnerAgentID != agentID {
			return status.Errorf(codes.PermissionDenied, "Local Agent %s input customer ownership mismatch", input.Role)
		}
		if key == "" || (key != prefix && !strings.HasPrefix(key, prefix+"/")) || path.Clean(key) != key {
			return status.Errorf(codes.PermissionDenied, "Local Agent %s input is outside the registered customer prefix", input.Role)
		}
		input.CustomerReference = fmt.Sprintf("customer-object://%d/%s", agentID, key)
	}
	return nil
}

func customerStoragePrefix(reference string) (string, error) {
	value := strings.TrimSpace(reference)
	parsed, err := url.Parse(value)
	if err != nil || strings.ToLower(parsed.Scheme) != "local-file" || parsed.User != nil ||
		(parsed.Host != "" && parsed.Host != "localhost") || parsed.RawQuery != "" || parsed.Fragment == "" ||
		strings.Contains(value, "%") || strings.ContainsAny(value, "\r\n@") {
		return "", status.Error(codes.FailedPrecondition, "customer storage reference is invalid")
	}
	prefix := strings.Trim(parsed.Fragment, "/")
	if prefix == "" || path.Clean(prefix) != prefix || strings.Contains(prefix, "..") {
		return "", status.Error(codes.FailedPrecondition, "customer storage prefix is invalid")
	}
	return prefix, nil
}

func parseGatewayCustomerObjectReference(reference string, agentID int64, prefix string) (string, error) {
	value := strings.TrimSpace(reference)
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "customer-object" || parsed.Host != strconv.FormatInt(agentID, 10) ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || strings.Contains(value, "%") {
		return "", status.Error(codes.InvalidArgument, "customer object reference is invalid")
	}
	key := strings.TrimPrefix(parsed.Path, "/")
	if key == "" || path.Clean(key) != key || (key != prefix && !strings.HasPrefix(key, prefix+"/")) ||
		value != fmt.Sprintf("customer-object://%d/%s", agentID, key) {
		return "", status.Error(codes.PermissionDenied, "customer object reference is outside the registered prefix")
	}
	return key, nil
}

func agentRPC(_ *svc.ServiceContext, call agentRPCFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeAgentError(w, status.Error(codes.Unimplemented, "method is not allowed"))
			return
		}
		identity, err := verifiedAgentIdentity(r)
		if err != nil {
			writeAgentError(w, err)
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2<<20))
		if err != nil {
			writeAgentError(w, status.Error(codes.InvalidArgument, "request body is invalid"))
			return
		}
		response, err := call(r.Context(), body, identity)
		writeAgentJSON(w, response, err)
	}
}

func verifiedAgentIdentity(r *http.Request) (agentTLSIdentity, error) {
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		return agentTLSIdentity{}, status.Error(codes.Unauthenticated, "verified client certificate is required")
	}
	certificate := r.TLS.PeerCertificates[0]
	digest := sha256.Sum256(certificate.Raw)
	return agentTLSIdentity{serial: strings.ToLower(certificate.SerialNumber.Text(16)), fingerprint: hex.EncodeToString(digest[:])}, nil
}

func injectAgentIdentity(auth *core.LocalAgentAuth, identity agentTLSIdentity) {
	if auth == nil {
		return
	}
	auth.CertificateSerial = identity.serial
	auth.CertificateFingerprintSha256 = identity.fingerprint
}

func decodeAgentJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 2<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func decodeAgentJSONBytes(body []byte, target any) error {
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.New("request body contains trailing data")
	}
	return nil
}

func decodeAgentRPCBody(body []byte, target any) error {
	if err := decodeAgentJSONBytes(body, target); err != nil {
		return status.Error(codes.InvalidArgument, "request body is invalid")
	}
	return nil
}

func workerAgentBase() *common.RespBase { return &common.RespBase{Code: 0, Msg: "success"} }

func writeAgentJSON(w http.ResponseWriter, value any, err error) {
	if err != nil {
		writeAgentError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
func writeAgentError(w http.ResponseWriter, err error) {
	code := status.Code(err)
	httpCode := http.StatusInternalServerError
	switch code {
	case codes.InvalidArgument:
		httpCode = http.StatusBadRequest
	case codes.Unauthenticated:
		httpCode = http.StatusUnauthorized
	case codes.PermissionDenied:
		httpCode = http.StatusForbidden
	case codes.NotFound:
		httpCode = http.StatusNotFound
	case codes.AlreadyExists:
		httpCode = http.StatusConflict
	case codes.ResourceExhausted:
		httpCode = http.StatusTooManyRequests
	case codes.FailedPrecondition:
		httpCode = http.StatusPreconditionFailed
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpCode)
	_ = json.NewEncoder(w).Encode(map[string]any{"code": code.String(), "message": status.Convert(err).Message()})
}
func clientIP(r *http.Request) string {
	if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); forwarded != "" {
		return forwarded
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
