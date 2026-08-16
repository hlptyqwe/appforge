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
	"strings"
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
	var taskID int64
	t.Cleanup(func() {
		if taskID > 0 {
			cleanupQueries := []string{
				`DELETE d FROM t_webhook_delivery d JOIN t_outbox_event o ON o.id=d.outbox_event_id WHERE o.aggregate_type='build' AND o.aggregate_id=?`,
				`DELETE FROM t_hybrid_artifact_reference WHERE task_id=?`,
				`DELETE FROM t_build_scheduler_event WHERE task_id=?`,
				`DELETE FROM t_build_slot_lease WHERE task_id=?`,
				`DELETE FROM t_usage_ledger WHERE resource_type='build' AND resource_id=?`,
				`DELETE FROM t_quota_reservation WHERE resource_type='build' AND resource_id=?`,
				`DELETE FROM t_outbox_event WHERE aggregate_type='build' AND aggregate_id=?`,
				`DELETE FROM t_build_task WHERE id=?`,
			}
			for _, query := range cleanupQueries {
				if _, cleanupErr := db.ExecCtx(context.Background(), query, taskID); cleanupErr != nil {
					t.Logf("cleanup Local Agent fault task: %v", cleanupErr)
				}
			}
		}
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
		RegistrationToken: registration.RegistrationToken, CsrPem: csr, AgentVersion: "1.1.0",
		ProtocolVersion: 3, Nonce: "registration-nonce-0001", Timestamp: registerTimestamp, SourceIp: "192.0.2.10",
	})
	if err != nil {
		t.Fatal(err)
	}
	if registered.Data.Status != core.LocalAgentStatus_LOCAL_AGENT_STATUS_ONLINE || registered.Certificate.CertificatePem == "" {
		t.Fatalf("unexpected registration result: %#v", registered)
	}
	if _, err := registerLocalAgent(context.Background(), svcCtx, &core.RegisterLocalAgentReq{
		RegistrationToken: registration.RegistrationToken, CsrPem: csr, AgentVersion: "1.1.0",
		ProtocolVersion: 3, Nonce: "registration-nonce-0002", Timestamp: registerTimestamp + 1,
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
		AgentVersion: "1.1.0", ProtocolVersion: 3, Capabilities: []*core.LocalAgentCapability{{CapabilityKey: "apk", CapabilityValue: "true"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := heartbeatLocalAgent(context.Background(), svcCtx, &core.HeartbeatLocalAgentReq{Auth: firstAuth,
		AgentVersion: "1.1.0", ProtocolVersion: 3}); status.Code(err) != codes.AlreadyExists {
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
		Auth: auth(registered.Certificate), AgentVersion: "1.1.0", ProtocolVersion: 3,
	}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("rotated certificate remained valid: code=%v err=%v", status.Code(err), err)
	}
	if _, err := heartbeatLocalAgent(context.Background(), svcCtx, &core.HeartbeatLocalAgentReq{
		Auth: auth(rotated.Certificate), AgentVersion: "1.1.0", ProtocolVersion: 3,
	}); err != nil {
		t.Fatalf("new certificate was not accepted: %v", err)
	}

	created, err := db.ExecCtx(context.Background(), `INSERT INTO t_build_task
(tenant_id,app_id,version_id,channel_id,signing_config_id,channel_code,version_code,version_name,
source_apk_object_id,pool_code,status,builder_attempt,priority,queued_at,create_by)
VALUES (?,?,?,?,?,'fault-injection',1,'1.0',1,?,'PENDING',0,1000,CURRENT_TIMESTAMP(3),0)`,
		application.TenantID, application.ID, 1, 1, 1, "enterprise-runtime")
	if err != nil {
		t.Fatal(err)
	}
	taskID, err = created.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecCtx(context.Background(), `INSERT INTO t_quota_reservation
(tenant_id,metric,quantity,resource_type,resource_id,idempotency_key,period_key,status,expires_at,confirmed_at)
VALUES (?,'build.count',1,'build',?,CONCAT('build:',?),DATE_FORMAT(CURRENT_DATE,'%Y-%m'),2,DATE_ADD(CURRENT_TIMESTAMP(3),INTERVAL 1 HOUR),CURRENT_TIMESTAMP(3))`,
		application.TenantID, taskID, taskID); err != nil {
		t.Fatal(err)
	}
	firstClaim, err := claimLocalAgentBuildTask(context.Background(), svcCtx, &core.ClaimLocalAgentBuildTaskReq{
		Auth: auth(rotated.Certificate), LeaseSeconds: 120,
	})
	if err != nil {
		t.Fatal(err)
	}
	if firstClaim.Task == nil || firstClaim.Task.Id != taskID || firstClaim.Task.BuilderAttempt != 1 {
		t.Fatalf("unexpected first Local Agent claim: %#v", firstClaim.Task)
	}
	if _, err := db.ExecCtx(context.Background(), `UPDATE t_build_task SET lease_until=DATE_SUB(CURRENT_TIMESTAMP(3),INTERVAL 1 SECOND) WHERE id=?`, taskID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecCtx(context.Background(), `UPDATE t_build_slot_lease SET lease_until=DATE_SUB(CURRENT_TIMESTAMP(3),INTERVAL 1 SECOND) WHERE task_id=? AND builder_attempt=?`, taskID, firstClaim.Task.BuilderAttempt); err != nil {
		t.Fatal(err)
	}
	secondClaim, err := claimLocalAgentBuildTask(context.Background(), svcCtx, &core.ClaimLocalAgentBuildTaskReq{
		Auth: auth(rotated.Certificate), LeaseSeconds: 120,
	})
	if err != nil {
		t.Fatal(err)
	}
	if secondClaim.Task == nil || secondClaim.Task.Id != taskID || secondClaim.Task.BuilderAttempt != 2 {
		t.Fatalf("expired Local Agent task was not fenced and recovered: %#v", secondClaim.Task)
	}
	if _, err := renewLocalAgentTaskLease(context.Background(), svcCtx, &core.RenewLocalAgentTaskLeaseReq{
		Auth: auth(rotated.Certificate), TaskId: taskID, BuilderAttempt: 1, LeaseSeconds: 120,
	}); status.Code(err) != codes.NotFound {
		t.Fatalf("old attempt renewed after recovery: code=%v err=%v", status.Code(err), err)
	}
	if _, err := completeLocalAgentBuildTask(context.Background(), svcCtx, &core.CompleteLocalAgentBuildTaskReq{
		Auth: auth(rotated.Certificate), TaskId: taskID, BuilderAttempt: 1,
		ApkReference: "local-artifact://fault/old.apk", ApkSha256: strings.Repeat("a", 64), ApkSize: 1,
	}); status.Code(err) != codes.NotFound {
		t.Fatalf("old attempt completed after recovery: code=%v err=%v", status.Code(err), err)
	}
	if _, err := failLocalAgentBuildTask(context.Background(), svcCtx, &core.FailLocalAgentBuildTaskReq{
		Auth: auth(rotated.Certificate), TaskId: taskID, BuilderAttempt: 2, ErrorMessage: "acceptance failure without log",
	}); err != nil {
		t.Fatalf("Local Agent failure without log was rejected: %v", err)
	}
	var failedTask struct {
		Status      string `db:"status"`
		LogObjectID int64  `db:"log_object_id"`
	}
	if err := db.QueryRowCtx(context.Background(), &failedTask, `SELECT status,log_object_id FROM t_build_task WHERE id=?`, taskID); err != nil {
		t.Fatal(err)
	}
	if failedTask.Status != buildStatusFailed || failedTask.LogObjectID != 0 {
		t.Fatalf("unexpected failed task without log: %#v", failedTask)
	}
	if _, err := drainLocalAgent(ctx, svcCtx, &core.DrainLocalAgentReq{Id: agentID,
		DrainStatus: core.LocalAgentDrainStatus_LOCAL_AGENT_DRAIN_STATUS_DRAINING}); err != nil {
		t.Fatal(err)
	}
	claimed, err := claimLocalAgentBuildTask(context.Background(), svcCtx, &core.ClaimLocalAgentBuildTaskReq{
		Auth: auth(rotated.Certificate), LeaseSeconds: 120,
	})
	if err != nil {
		t.Fatal(err)
	}
	if claimed.Task != nil {
		t.Fatal("draining Local Agent claimed a new task")
	}
	if _, err := revokeLocalAgent(ctx, svcCtx, &core.RevokeLocalAgentReq{Id: agentID, Reason: "acceptance completed"}); err != nil {
		t.Fatal(err)
	}
	if _, err := heartbeatLocalAgent(context.Background(), svcCtx, &core.HeartbeatLocalAgentReq{
		Auth: auth(rotated.Certificate), AgentVersion: "1.1.0", ProtocolVersion: 3,
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
