// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"
	"fmt"
	"sync"
	"time"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	"appforge/common/offlinelicense"
	"appforge/common/utils"
	"appforge/proto/system"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	grpcstatus "google.golang.org/grpc/status"
)

type GetPlatformDeploymentStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetPlatformDeploymentStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPlatformDeploymentStatusLogic {
	return &GetPlatformDeploymentStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPlatformDeploymentStatusLogic) GetPlatformDeploymentStatus() (resp *types.PlatformDeploymentStatusResp, err error) {
	userType, userTypeErr := utils.GetUserTypeFromCtx(l.ctx)
	if userTypeErr != nil || userType != utils.SysUserTypeSystemAdmin {
		return nil, grpcstatus.Error(codes.PermissionDenied, "system administrator is required")
	}
	checkedAt := time.Now().UnixMilli()
	components := []types.PlatformDeploymentComponent{
		{Code: "api", Name: "Admin API", Status: "healthy", CheckedAt: checkedAt},
		{Code: "system-rpc", Name: "System RPC", Status: "checking", CheckedAt: checkedAt},
		{Code: "core-rpc", Name: "Core RPC", Status: "checking", CheckedAt: checkedAt},
		{Code: "builder-rpc", Name: "Builder RPC", Status: "checking", CheckedAt: checkedAt},
		{Code: "database", Name: "MySQL / Schema", Status: "checking", CheckedAt: checkedAt},
	}

	type probe struct {
		index  int
		client grpc_health_v1.HealthClient
	}
	probes := []probe{{1, l.svcCtx.SystemHealth}, {2, l.svcCtx.CoreHealth}, {3, l.svcCtx.BuilderHealth}}
	var wait sync.WaitGroup
	for _, item := range probes {
		wait.Add(1)
		go func(item probe) {
			defer wait.Done()
			components[item.index] = checkRPCHealth(l.ctx, components[item.index], item.client)
		}(item)
	}

	databaseContext, cancel := context.WithTimeout(l.ctx, 3*time.Second)
	databaseStatus, databaseErr := l.svcCtx.SystemCli.GetDeploymentDatabaseStatus(databaseContext, &system.Empty{})
	cancel()
	actualSchemaVersion := ""
	migrationCount := int64(0)
	migrations := make([]types.PlatformDeploymentMigration, 0)
	if databaseErr != nil {
		components[4].Status = "unhealthy"
		components[4].Message = fmt.Sprintf("迁移状态查询失败：%s", grpcstatus.Code(databaseErr))
	} else if databaseStatus.GetBase().GetCode() != 200 {
		components[4].Status = "unhealthy"
		components[4].Message = "迁移状态查询未成功"
	} else {
		components[4].Status = "healthy"
		actualSchemaVersion = databaseStatus.GetLatestSchemaVersion()
		migrationCount = databaseStatus.GetMigrationCount()
		for _, item := range databaseStatus.GetRecentMigrations() {
			migrations = append(migrations, types.PlatformDeploymentMigration{
				Version: item.GetVersion(), Description: item.GetDescription(),
			})
		}
	}
	wait.Wait()

	deployment := l.svcCtx.Config.Deployment
	licenseStatus, licenseReady := publicLicenseStatus(l.svcCtx.License, l.svcCtx.Config.OfflineLicense.Enabled, time.Now())
	schemaCompatible := actualSchemaVersion != "" && actualSchemaVersion == deployment.TargetSchemaVersion
	upgradeReady := schemaCompatible && licenseReady
	for _, component := range components {
		if component.Status != "healthy" {
			upgradeReady = false
			break
		}
	}

	return &types.PlatformDeploymentStatusResp{
		RespBase: types.RespBase{Code: 200, Msg: "OK"},
		Data: types.PlatformDeploymentStatus{
			DeploymentId: deployment.DeploymentId, DeploymentMode: deployment.DeploymentMode,
			ProductVersion: deployment.ProductVersion, TargetSchemaVersion: deployment.TargetSchemaVersion,
			ActualSchemaVersion: actualSchemaVersion, SchemaCompatible: schemaCompatible,
			MaxVersionSkew: deployment.MaxVersionSkew, AgentProtocolCurrent: deployment.AgentProtocolCurrent,
			AgentProtocolMinimum: deployment.AgentProtocolMinimum, UpgradeReady: upgradeReady,
			DiagnosticsCommand: "deploy/production/diagnostics.sh OUTPUT.tar.gz",
			MigrationCount:     migrationCount, RecentMigrations: migrations, Components: components,
			License: licenseStatus, CheckedAt: checkedAt,
		},
	}, nil
}

func checkRPCHealth(ctx context.Context, component types.PlatformDeploymentComponent, client grpc_health_v1.HealthClient) types.PlatformDeploymentComponent {
	if client == nil {
		component.Status = "unhealthy"
		component.Message = "健康客户端未初始化"
		return component
	}
	probeContext, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	response, err := client.Check(probeContext, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		component.Status = "unhealthy"
		component.Message = fmt.Sprintf("gRPC 健康探测失败：%s", grpcstatus.Code(err))
		return component
	}
	if response.GetStatus() != grpc_health_v1.HealthCheckResponse_SERVING {
		component.Status = "degraded"
		component.Message = "gRPC 服务未处于 SERVING 状态"
		return component
	}
	component.Status = "healthy"
	return component
}

func publicLicenseStatus(license *offlinelicense.VerifiedLicense, enabled bool, now time.Time) (types.PlatformDeploymentLicense, bool) {
	if !enabled {
		return types.PlatformDeploymentLicense{Enabled: false, Status: "not_required"}, true
	}
	if license == nil {
		return types.PlatformDeploymentLicense{Enabled: true, Status: "invalid"}, false
	}
	payload := license.Payload
	result := types.PlatformDeploymentLicense{
		Enabled: true, Status: "valid", LicenseId: payload.LicenseID, Customer: payload.Customer,
		DeploymentId: payload.DeploymentID, DeploymentModes: append([]string(nil), payload.DeploymentModes...),
		Features: append([]string(nil), payload.Features...), NotBefore: payload.NotBefore, NotAfter: payload.NotAfter,
		Sequence: payload.Sequence, MaxTenants: payload.MaxTenants, MaxBuilders: payload.MaxBuilders,
		Fingerprint: license.Fingerprint,
	}
	if err := license.ValidAt(now); err != nil {
		result.Status = "invalid"
		return result, false
	}
	return result, true
}
