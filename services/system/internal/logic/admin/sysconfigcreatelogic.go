package adminlogic

import (
	"context"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type SysConfigCreateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSysConfigCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysConfigCreateLogic {
	return &SysConfigCreateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SysConfigCreateLogic) SysConfigCreate(in *system.SysConfigCreateReq) (*system.RespBase, error) {
	// todo: add your logic here and delete this line

	return &system.RespBase{}, nil
}
