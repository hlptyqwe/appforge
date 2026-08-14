// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package open_v1

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpenV1ListVersionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpenV1ListVersionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenV1ListVersionsLogic {
	return &OpenV1ListVersionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpenV1ListVersionsLogic) OpenV1ListVersions(req *types.ListPlatformVersionsReq) (resp *types.PlatformVersionListResp, err error) {
	return openV1ListVersions(l.ctx, l.svcCtx, req)
}
