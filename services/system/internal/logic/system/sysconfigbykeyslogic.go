package systemlogic

import (
	"context"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
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
	// todo: add your logic here and delete this line

	return &system.SysConfigByKeysResp{}, nil
}
