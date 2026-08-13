package adminlogic

import (
	"context"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type Google2FAEnableLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGoogle2FAEnableLogic(ctx context.Context, svcCtx *svc.ServiceContext) *Google2FAEnableLogic {
	return &Google2FAEnableLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *Google2FAEnableLogic) Google2FAEnable(in *system.Google2FAEnableReq) (*system.RespBase, error) {
	// todo: add your logic here and delete this line

	return &system.RespBase{}, nil
}
