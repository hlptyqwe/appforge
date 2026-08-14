// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package open_v1

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpenV1GetBuildLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpenV1GetBuildLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenV1GetBuildLogic {
	return &OpenV1GetBuildLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpenV1GetBuildLogic) OpenV1GetBuild(req *types.PlatformIdReq) (resp *types.PlatformBuildTaskResp, err error) {
	return openV1GetBuild(l.ctx, l.svcCtx, req)
}
