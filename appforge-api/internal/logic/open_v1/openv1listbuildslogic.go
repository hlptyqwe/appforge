// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package open_v1

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpenV1ListBuildsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpenV1ListBuildsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenV1ListBuildsLogic {
	return &OpenV1ListBuildsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpenV1ListBuildsLogic) OpenV1ListBuilds(req *types.ListPlatformBuildTasksReq) (resp *types.PlatformBuildTaskListResp, err error) {
	return openV1ListBuilds(l.ctx, l.svcCtx, req)
}
