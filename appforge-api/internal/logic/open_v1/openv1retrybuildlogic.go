// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package open_v1

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpenV1RetryBuildLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpenV1RetryBuildLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenV1RetryBuildLogic {
	return &OpenV1RetryBuildLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpenV1RetryBuildLogic) OpenV1RetryBuild(req *types.RetryPlatformBuildTaskReq) (resp *types.PlatformBuildTaskResp, err error) {
	return openV1RetryBuild(l.ctx, l.svcCtx, req)
}
