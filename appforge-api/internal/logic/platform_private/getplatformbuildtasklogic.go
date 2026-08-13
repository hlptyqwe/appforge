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

type GetPlatformBuildTaskLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetPlatformBuildTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPlatformBuildTaskLogic {
	return &GetPlatformBuildTaskLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPlatformBuildTaskLogic) GetPlatformBuildTask(req *types.PlatformIdReq) (resp *types.PlatformBuildTaskResp, err error) {
	item, err := l.svcCtx.CoreCli.GetBuildTask(l.ctx, &core.BuildTaskIdReq{Id: req.Id})
	if err != nil {
		return nil, err
	}
	return &types.PlatformBuildTaskResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: platformlogic.MapPlatformBuildTask(item.Data)}, nil
}
