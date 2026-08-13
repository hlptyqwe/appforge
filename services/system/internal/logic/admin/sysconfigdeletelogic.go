package adminlogic

import (
	"context"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type SysConfigDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSysConfigDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysConfigDeleteLogic {
	return &SysConfigDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SysConfigDeleteLogic) SysConfigDelete(in *system.SysConfigDeleteReq) (*system.RespBase, error) {
	// todo: add your logic here and delete this line

	return &system.RespBase{}, nil
}
