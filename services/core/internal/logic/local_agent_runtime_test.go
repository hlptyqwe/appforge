package logic

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"os"
	"testing"
	"time"

	"appforge/common/utils"
	"appforge/proto/core"
	"appforge/services/core/internal/agentpki"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestLocalAgentRuntimeAcceptance(t *testing.T) {
	dsn := os.Getenv("APPFORGE_ENTERPRISE_TEST_DSN")
	if dsn == "" {
		t.Skip("APPFORGE_ENTERPRISE_TEST_DSN is not set")
	}
	db := sqlx.NewMysql(dsn)
	var application struct {
		ID       int64 `db:"id"`
		TenantID int64 `db:"tenant_id"`
	}
	if err := db.QueryRowCtx(context.Background(), &application, `SELECT id,tenant_id FROM t_app_application WHERE status=1 ORDER BY id LIMIT 1`); err != nil {
		t.Fatalf("load acceptance application: %v", err)
	}
	pki, err := agentpki.New("", "", 2*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	svcCtx := &svc.ServiceContext{DB: db, AgentPKI: pki}
	ctx := context.WithValue(context.Background(), utils.CtxKeyTenantId, application.TenantID)
	code := fmt.Sprintf("v7-runtime-%d", time.Now().UnixNano())
	var agentID int64
	t.Cleanup(func() {
		if agentID <= 0 {
			return
		}
		for _, query := range []string{
			`DELETE FROM t_hybrid_artifact_reference WHERE agent_id=?`,
			`DELETE FROM t_local_agent_capability WHERE agent_id=?`,
			`DELETE FROM t_local_agent_certificate WHERE agent_id=?`,
			`DELETE FROM t_local_agent_registration WHERE agent_id=?`,
			`DELETE FROM t_local_agent WHERE id=?`,
		} {
			if _, cleanupErr := db.ExecCtx(context.Background(), query, agentID); cleanupErr != nil {
				t.Logf("cleanup Local Agent: %v", cleanupErr)
			}
		}
	})

	registration, err := createLocalAgentRegistration(ctx, svcCtx, &core.CreateLocalAgentRegistrationReq{
		AgentCode: code, AgentName: "V7 runtime acceptance", PoolCode: "enterprise-runtime",
		ArtifactMode:  core.HybridArtifactMode_HYBRID_ARTIFACT_MODE_CONTROL_PLANE_STORAGE,
		AllowedAppIds: []int64{application.ID}, Capabilities: []*core.LocalAgentCapability{
			{CapabilityKey: "apk", CapabilityValue: "true"},
			{CapabilityKey: "max_concurrency", CapabilityValue: "1"},
		}, ExpiresSeconds: 300,
	})
	if err != nil {
		t.Fatal(err)
	}
	agentID = registration.Data.Id
	_, csr := acceptanceAgentCSR(t)
	registerTimestamp := time.Now().UnixMilli()
	registered, err := registerLocalAgent(context.Background(), svcCtx, &core.RegisterLocalAgentReq{
		RegistrationToken: registration.RegistrationToken, CsrPem: csr, AgentVersion: "1.0.0",
		ProtocolVersion: 2, Nonce: "registration-nonce-0001", Timestamp: registerTimestamp, SourceIp: "192.0.2.10",
	})
	if err != nil {
		t.Fatal(err)
	}
	if registered.Data.Status != core.LocalAgentStatus_LOCAL_AGENT_STATUS_ONLINE || registered.Certificate.CertificatePem == "" {
		t.Fatalf("unexpected registration result: %#v", registered)
	}
	if _, err := registerLocalAgent(context.Background(), svcCtx, &core.RegisterLocalAgentReq{
		RegistrationToken: registration.RegistrationToken, CsrPem: csr, AgentVersion: "1.0.0",
		ProtocolVersion: 2, Nonce: "registration-nonce-0002", Timestamp: registerTimestamp + 1,
	}); err == nil {
		t.Fatal("one-time registration token was accepted twice")
	}

	sequence := registerTimestamp + 10
	auth := func(certificate *core.LocalAgentCertificate) *core.LocalAgentAuth {
		sequence++
		return &core.LocalAgentAuth{AgentId: agentID, CertificateSerial: certificate.SerialNumber,
			CertificateFingerprintSha256: certificate.FingerprintSha256,
			Nonce:                        fmt.Sprintf("runtime-nonce-%016d", sequence), Timestamp: sequence}
	}
	firstAuth := auth(registered.Certificate)
	if _, err := heartbeatLocalAgent(context.Background(), svcCtx, &core.HeartbeatLocalAgentReq{Auth: firstAuth,
		AgentVersion: "1.0.0", ProtocolVersion: 2, Capabilities: []*core.LocalAgentCapability{{CapabilityKey: "apk", CapabilityValue: "true"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := heartbeatLocalAgent(context.Background(), svcCtx, &core.HeartbeatLocalAgentReq{Auth: firstAuth,
		AgentVersion: "1.0.0", ProtocolVersion: 2}); status.Code(err) != codes.AlreadyExists {
		t.Fatalf("replayed nonce/timestamp code = %v, err=%v", status.Code(err), err)
	}

	_, rotatedCSR := acceptanceAgentCSR(t)
	rotated, err := rotateLocalAgentCertificate(context.Background(), svcCtx, &core.RotateLocalAgentCertificateReq{
		Auth: auth(registered.Certificate), CsrPem: rotatedCSR,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rotated.Certificate.SerialNumber == registered.Certificate.SerialNumber {
		t.Fatal("certificate rotation reused the old serial")
	}
	if _, err := heartbeatLocalAgent(context.Background(), svcCtx, &core.HeartbeatLocalAgentReq{
		Auth: auth(registered.Certificate), AgentVersion: "1.0.0", ProtocolVersion: 2,
	}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("rotated certificate remained valid: code=%v err=%v", status.Code(err), err)
	}
	if _, err := heartbeatLocalAgent(context.Background(), svcCtx, &core.HeartbeatLocalAgentReq{
		Auth: auth(rotated.Certificate), AgentVersion: "1.0.1", ProtocolVersion: 2,
	}); err != nil {
		t.Fatalf("new certificate was not accepted: %v", err)
	}
	if _, err := drainLocalAgent(ctx, svcCtx, &core.DrainLocalAgentReq{Id: agentID,
		DrainStatus: core.LocalAgentDrainStatus_LOCAL_AGENT_DRAIN_STATUS_DRAINING}); err != nil {
		t.Fatal(err)
	}
	if _, err := revokeLocalAgent(ctx, svcCtx, &core.RevokeLocalAgentReq{Id: agentID, Reason: "acceptance completed"}); err != nil {
		t.Fatal(err)
	}
	if _, err := heartbeatLocalAgent(context.Background(), svcCtx, &core.HeartbeatLocalAgentReq{
		Auth: auth(rotated.Certificate), AgentVersion: "1.0.1", ProtocolVersion: 2,
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("revoked Agent was accepted: code=%v err=%v", status.Code(err), err)
	}
}

func acceptanceAgentCSR(t *testing.T) (*ecdsa.PrivateKey, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: "v7-runtime-agent"},
	}, key)
	if err != nil {
		t.Fatal(err)
	}
	return key, string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der}))
}
