// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package system

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"appforge/admin-api/internal/logicutil"

	"github.com/zeromicro/go-zero/core/logx"
)

type Google2FAResetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGoogle2FAResetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *Google2FAResetLogic {
	return &Google2FAResetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *Google2FAResetLogic) Google2FAReset(req *types.Google2FAResetReq) (resp *types.RespBase, err error) {
	return logicutil.Proxy[types.RespBase](l.ctx, req, l.svcCtx.SystemCli.Google2FAReset)
}
