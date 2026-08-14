package adminlogic

import (
	"context"
	"fmt"
	"strings"
	"time"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SysTenantCreateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSysTenantCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysTenantCreateLogic {
	return &SysTenantCreateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SysTenantCreateLogic) SysTenantCreate(in *system.SysTenantCreateReq) (*system.RespBase, error) {
	if in == nil || strings.TrimSpace(in.GetUsername()) == "" || strings.TrimSpace(in.GetTenantName()) == "" || in.GetTenantPassword() == "" {
		return nil, status.Error(codes.InvalidArgument, "username, tenant_name and tenant_password are required")
	}
	if tenantID(l.ctx) > 0 {
		return nil, status.Error(codes.PermissionDenied, "only system administrators can manage tenants")
	}
	password, err := bcrypt.GenerateFromPassword([]byte(in.GetTenantPassword()), bcrypt.DefaultCost)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "hash tenant password failed: %v", err)
	}
	now := time.Now().UnixMilli()
	code := strings.ToLower(strings.TrimSpace(in.GetUsername()))
	code = strings.NewReplacer(" ", "_", "/", "_", "\\", "_").Replace(code)
	if len(code) > 64 {
		code = code[:64]
	}
	if code == "" {
		code = fmt.Sprintf("tenant_%d", now)
	}
	enabled := int64(in.GetEnabled())
	if enabled == 0 {
		enabled = 1
	}
	actor := actorID(l.ctx)
	tenantName := strings.TrimSpace(in.GetTenantName())
	username := strings.TrimSpace(in.GetUsername())
	appScope := int64(system.ApplicationScope_APPLICATION_SCOPE_AGENT)
	err = l.svcCtx.DB.TransactCtx(l.ctx, func(txCtx context.Context, session sqlx.Session) error {
		tenantResult, execErr := session.ExecCtx(txCtx, `INSERT INTO sys_tenant
(tenant_code, tenant_name, enabled, expire_time, contact_name, contact_phone, remark, create_by, create_times, update_by, update_times)
VALUES (?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, ?)`,
			code, tenantName, enabled, in.GetExpireTime(), strings.TrimSpace(in.GetContactName()),
			strings.TrimSpace(in.GetContactPhone()), strings.TrimSpace(in.GetRemark()), fmt.Sprintf("%d", actor), now,
			fmt.Sprintf("%d", actor), now)
		if execErr != nil {
			return fmt.Errorf("create tenant: %w", execErr)
		}
		tenantID, execErr := tenantResult.LastInsertId()
		if execErr != nil {
			return fmt.Errorf("resolve created tenant id: %w", execErr)
		}
		if tenantID <= 0 {
			return fmt.Errorf("resolve created tenant id: non-positive id")
		}

		ownerResult, execErr := session.ExecCtx(txCtx, `INSERT INTO sys_user
(tenant_id, app_scope, user_type, is_owner, username, password, nickname, avatar, enabled, google_secret,
 google_enabled, perms_ver, last_login_ip, last_login_at, create_by, create_times, update_times)
VALUES (?, ?, ?, 1, ?, ?, ?, '', 1, '', 2, 1, '', 0, ?, ?, ?)`,
			tenantID, appScope, int64(system.UserType_USER_TYPE_TENANT_OWNER), username, string(password), tenantName, actor, now, now)
		if execErr != nil {
			return fmt.Errorf("create tenant owner: %w", execErr)
		}
		ownerID, execErr := ownerResult.LastInsertId()
		if execErr != nil {
			return fmt.Errorf("resolve tenant owner id: %w", execErr)
		}
		if ownerID <= 0 {
			return fmt.Errorf("resolve tenant owner id: non-positive id")
		}

		roleResult, execErr := session.ExecCtx(txCtx, `INSERT INTO sys_role
(tenant_id, app_scope, name, code, enabled, remark, create_times, update_times)
VALUES (?, ?, '所有者', 'owner', 1, '代理端租户所有者', ?, ?)`, tenantID, appScope, now, now)
		if execErr != nil {
			return fmt.Errorf("create tenant owner role: %w", execErr)
		}
		roleID, execErr := roleResult.LastInsertId()
		if execErr != nil {
			return fmt.Errorf("resolve tenant owner role id: %w", execErr)
		}
		if roleID <= 0 {
			return fmt.Errorf("resolve tenant owner role id: non-positive id")
		}
		if _, execErr = session.ExecCtx(txCtx,
			`INSERT INTO sys_user_role (tenant_id, user_id, role_id) VALUES (?, ?, ?)`, tenantID, ownerID, roleID); execErr != nil {
			return fmt.Errorf("assign tenant owner role: %w", execErr)
		}
		if _, execErr = session.ExecCtx(txCtx, `INSERT INTO sys_role_menu (tenant_id, role_id, menu_id)
SELECT ?, ?, id FROM sys_menu WHERE app_scope = ?`, tenantID, roleID, appScope); execErr != nil {
			return fmt.Errorf("grant tenant owner permissions: %w", execErr)
		}
		return nil
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create tenant transaction failed: %v", err)
	}
	return &system.RespBase{Base: responseBase()}, nil
}
