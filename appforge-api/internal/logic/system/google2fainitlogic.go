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

type Google2FAInitLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGoogle2FAInitLogic(ctx context.Context, svcCtx *svc.ServiceContext) *Google2FAInitLogic {
	return &Google2FAInitLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *Google2FAInitLogic) Google2FAInit(req *types.Google2FAInitReq) (resp *types.Google2FAInitResp, err error) {
	return logicutil.Proxy[types.Google2FAInitResp](l.ctx, req, l.svcCtx.SystemCli.Google2FAInit)
}
