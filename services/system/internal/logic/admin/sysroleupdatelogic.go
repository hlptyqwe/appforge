package adminlogic

import (
	"context"
	"strings"
	"time"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SysRoleUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSysRoleUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysRoleUpdateLogic {
	return &SysRoleUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SysRoleUpdateLogic) SysRoleUpdate(in *system.SysRoleUpdateReq) (*system.RespBase, error) {
	if in == nil || in.GetId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	item, err := l.svcCtx.RoleModel.FindOne(l.ctx, in.GetId())
	if err != nil {
		return nil, notFound(err, "role")
	}
	if err := requireItemAppScope(l.ctx, item.AppScope); err != nil {
		return nil, err
	}
	if _, err := effectiveTenant(l.ctx, item.TenantId); err != nil {
		return nil, err
	}
	if in.GetName() != "" {
		item.Name = strings.TrimSpace(in.GetName())
	}
	if in.GetCode() != "" {
		item.Code = strings.TrimSpace(in.GetCode())
	}
	if in.GetEnabled() != 0 {
		item.Enabled = int64(in.GetEnabled())
	}
	if in.GetRemark() != "" {
		item.Remark = strings.TrimSpace(in.GetRemark())
	}
	item.UpdateTimes = time.Now().UnixMilli()
	if err := l.svcCtx.RoleModel.Update(l.ctx, item); err != nil {
		return nil, status.Errorf(codes.Internal, "update role failed: %v", err)
	}
	return &system.RespBase{Base: responseBase()}, nil
}
