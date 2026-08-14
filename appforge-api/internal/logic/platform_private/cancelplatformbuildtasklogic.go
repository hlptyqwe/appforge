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

type CancelPlatformBuildTaskLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCancelPlatformBuildTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CancelPlatformBuildTaskLogic {
	return &CancelPlatformBuildTaskLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CancelPlatformBuildTaskLogic) CancelPlatformBuildTask(req *types.CancelPlatformBuildTaskReq) (resp *types.PlatformBuildTaskResp, err error) {
	item, err := l.svcCtx.CoreCli.CancelBuildTask(l.ctx, &core.CancelBuildTaskReq{Id: req.Id, Reason: req.Reason})
	if err != nil {
		return nil, err
	}
	return &types.PlatformBuildTaskResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: platformlogic.MapPlatformBuildTask(item.Data)}, nil
}
