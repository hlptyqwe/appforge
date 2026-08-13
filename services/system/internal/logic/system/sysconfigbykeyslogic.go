package systemlogic

import (
	"context"
	"database/sql"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SysConfigByKeysLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSysConfigByKeysLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysConfigByKeysLogic {
	return &SysConfigByKeysLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SysConfigByKeysLogic) SysConfigByKeys(in *system.SysConfigByKeysReq) (*system.SysConfigByKeysResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	tenant := tenantID(l.ctx)
	data := make([]*system.SysConfigItem, 0, len(in.GetConfigKeys()))
	for _, key := range in.GetConfigKeys() {
		key = trim(key)
		if key == "" {
			continue
		}
		item, err := l.svcCtx.ConfigModel.FindOneByTenantIdConfigKey(l.ctx, tenant, sql.NullString{String: key, Valid: true})
		if err != nil {
			continue
		}
		data = append(data, configItem(item))
	}
	return &system.SysConfigByKeysResp{Base: responseBase(), Data: data}, nil
}
