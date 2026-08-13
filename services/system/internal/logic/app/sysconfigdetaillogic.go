package applogic

import (
	"context"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
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
	// todo: add your logic here and delete this line

	return &system.SysConfigDetailResp{}, nil
}
