// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package open_v1

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpenV1CreateBuildLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpenV1CreateBuildLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenV1CreateBuildLogic {
	return &OpenV1CreateBuildLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpenV1CreateBuildLogic) OpenV1CreateBuild(req *types.CreatePlatformBuildTaskReq) (resp *types.PlatformBuildTaskResp, err error) {
	return openV1CreateBuild(l.ctx, l.svcCtx, req)
}
