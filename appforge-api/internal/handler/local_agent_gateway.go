package handler

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"appforge/admin-api/internal/config"
	"appforge/admin-api/internal/svc"
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
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/heartbeat", agentRPC(svcCtx, func(ctx context.Context, body []byte, identity agentTLSIdentity) (any, error) {
		var req core.HeartbeatLocalAgentReq
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}
		injectAgentIdentity(req.Auth, identity)
		return svcCtx.CoreCli.HeartbeatLocalAgent(ctx, &req)
	}))
	mux.HandleFunc("/v1/claim", agentRPC(svcCtx, func(ctx context.Context, body []byte, identity agentTLSIdentity) (any, error) {
		var req core.ClaimLocalAgentBuildTaskReq
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}
		injectAgentIdentity(req.Auth, identity)
		return svcCtx.CoreCli.ClaimLocalAgentBuildTask(ctx, &req)
	}))
	mux.HandleFunc("/v1/tasks/renew", agentRPC(svcCtx, func(ctx context.Context, body []byte, identity agentTLSIdentity) (any, error) {
		var req core.RenewLocalAgentTaskLeaseReq
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}
		injectAgentIdentity(req.Auth, identity)
		return svcCtx.CoreCli.RenewLocalAgentTaskLease(ctx, &req)
	}))
	mux.HandleFunc("/v1/tasks/progress", agentRPC(svcCtx, func(ctx context.Context, body []byte, identity agentTLSIdentity) (any, error) {
		var req core.ReportLocalAgentBuildProgressReq
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}
		injectAgentIdentity(req.Auth, identity)
		return svcCtx.CoreCli.ReportLocalAgentBuildProgress(ctx, &req)
	}))
	mux.HandleFunc("/v1/tasks/complete", agentRPC(svcCtx, func(ctx context.Context, body []byte, identity agentTLSIdentity) (any, error) {
		var req core.CompleteLocalAgentBuildTaskReq
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}
		injectAgentIdentity(req.Auth, identity)
		return svcCtx.CoreCli.CompleteLocalAgentBuildTask(ctx, &req)
	}))
	mux.HandleFunc("/v1/tasks/fail", agentRPC(svcCtx, func(ctx context.Context, body []byte, identity agentTLSIdentity) (any, error) {
		var req core.FailLocalAgentBuildTaskReq
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}
		injectAgentIdentity(req.Auth, identity)
		return svcCtx.CoreCli.FailLocalAgentBuildTask(ctx, &req)
	}))
	mux.HandleFunc("/v1/artifacts/verify", agentRPC(svcCtx, func(ctx context.Context, body []byte, identity agentTLSIdentity) (any, error) {
		var req core.VerifyHybridArtifactReq
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}
		injectAgentIdentity(req.Auth, identity)
		return svcCtx.CoreCli.VerifyHybridArtifact(ctx, &req)
	}))
	mux.HandleFunc("/v1/certificates/rotate", agentRPC(svcCtx, func(ctx context.Context, body []byte, identity agentTLSIdentity) (any, error) {
		var req core.RotateLocalAgentCertificateReq
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}
		injectAgentIdentity(req.Auth, identity)
		return svcCtx.CoreCli.RotateLocalAgentCertificate(ctx, &req)
	}))
	server := &http.Server{Addr: cfg.ListenOn, Handler: mux, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 90 * time.Second,
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

func agentRPC(_ *svc.ServiceContext, call agentRPCFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeAgentError(w, status.Error(codes.Unimplemented, "method is not allowed"))
			return
		}
		if r.TLS == nil || len(r.TLS.PeerCertificates) != 1 {
			writeAgentError(w, status.Error(codes.Unauthenticated, "exactly one verified client certificate is required"))
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2<<20))
		if err != nil {
			writeAgentError(w, status.Error(codes.InvalidArgument, "request body is invalid"))
			return
		}
		certificate := r.TLS.PeerCertificates[0]
		digest := sha256.Sum256(certificate.Raw)
		response, err := call(r.Context(), body, agentTLSIdentity{serial: strings.ToLower(certificate.SerialNumber.Text(16)), fingerprint: hex.EncodeToString(digest[:])})
		writeAgentJSON(w, response, err)
	}
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
