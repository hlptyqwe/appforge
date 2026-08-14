// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package open_v1

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpenV1GetApplicationLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpenV1GetApplicationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenV1GetApplicationLogic {
	return &OpenV1GetApplicationLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpenV1GetApplicationLogic) OpenV1GetApplication(req *types.PlatformIdReq) (resp *types.PlatformApplicationResp, err error) {
	return openV1GetApplication(l.ctx, l.svcCtx, req)
}
