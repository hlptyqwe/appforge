// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package open_v1

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpenV1CreateVersionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpenV1CreateVersionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenV1CreateVersionLogic {
	return &OpenV1CreateVersionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpenV1CreateVersionLogic) OpenV1CreateVersion(req *types.CreatePlatformVersionReq) (resp *types.PlatformVersionResp, err error) {
	return openV1CreateVersion(l.ctx, l.svcCtx, req)
}
