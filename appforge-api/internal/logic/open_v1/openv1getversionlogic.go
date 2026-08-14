// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package open_v1

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpenV1GetVersionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpenV1GetVersionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenV1GetVersionLogic {
	return &OpenV1GetVersionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpenV1GetVersionLogic) OpenV1GetVersion(req *types.PlatformIdReq) (resp *types.PlatformVersionResp, err error) {
	return openV1GetVersion(l.ctx, l.svcCtx, req)
}
