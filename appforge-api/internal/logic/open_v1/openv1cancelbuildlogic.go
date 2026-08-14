// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package open_v1

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpenV1CancelBuildLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpenV1CancelBuildLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenV1CancelBuildLogic {
	return &OpenV1CancelBuildLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpenV1CancelBuildLogic) OpenV1CancelBuild(req *types.CancelPlatformBuildTaskReq) (resp *types.PlatformBuildTaskResp, err error) {
	return openV1CancelBuild(l.ctx, l.svcCtx, req)
}
