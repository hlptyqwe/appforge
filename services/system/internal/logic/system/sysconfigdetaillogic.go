package systemlogic

import (
	"context"
	"database/sql"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"
	"appforge/services/system/models"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SysConfigDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSysConfigDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysConfigDetailLogic {
	return &SysConfigDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SysConfigDetailLogic) SysConfigDetail(in *system.SysConfigDetailReq) (*system.SysConfigDetailResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	var (
		item *models.SysConfig
		err  error
	)
	if in.Id != nil {
		item, err = l.svcCtx.ConfigModel.FindOne(l.ctx, in.GetId())
	} else if in.ConfigKey != nil {
		tenant := tenantID(l.ctx)
		if in.TenantId != nil {
			tenant = in.GetTenantId()
		}
		item, err = l.svcCtx.ConfigModel.FindOneByTenantIdConfigKey(l.ctx, tenant, sql.NullString{String: in.GetConfigKey().String(), Valid: true})
	} else {
		return nil, status.Error(codes.InvalidArgument, "id or config_key is required")
	}
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "system config not found")
	}
	return &system.SysConfigDetailResp{Base: responseBase(), Data: configItem(item)}, nil
}
