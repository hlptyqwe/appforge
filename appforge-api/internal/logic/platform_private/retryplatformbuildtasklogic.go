// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	platformlogic "appforge/admin-api/internal/logic"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	"appforge/proto/core"

	"github.com/zeromicro/go-zero/core/logx"
)

type RetryPlatformBuildTaskLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRetryPlatformBuildTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RetryPlatformBuildTaskLogic {
	return &RetryPlatformBuildTaskLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RetryPlatformBuildTaskLogic) RetryPlatformBuildTask(req *types.RetryPlatformBuildTaskReq) (resp *types.PlatformBuildTaskResp, err error) {
	item, err := l.svcCtx.CoreCli.RetryBuildTask(l.ctx, &core.RetryBuildTaskReq{Id: req.Id, Priority: req.Priority})
	if err != nil {
		return nil, err
	}
	return &types.PlatformBuildTaskResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: platformlogic.MapPlatformBuildTask(item.Data)}, nil
}
