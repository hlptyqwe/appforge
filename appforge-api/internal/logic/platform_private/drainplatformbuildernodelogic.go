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

type DrainPlatformBuilderNodeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDrainPlatformBuilderNodeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DrainPlatformBuilderNodeLogic {
	return &DrainPlatformBuilderNodeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DrainPlatformBuilderNodeLogic) DrainPlatformBuilderNode(req *types.DrainPlatformBuilderNodeReq) (resp *types.PlatformBuilderNodeResp, err error) {
	item, err := l.svcCtx.CoreCli.DrainBuilderNode(l.ctx, &core.DrainBuilderNodeReq{Id: req.Id, DrainStatus: core.BuilderDrainStatus(req.DrainStatus)})
	if err != nil {
		return nil, err
	}
	return &types.PlatformBuilderNodeResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: platformlogic.MapPlatformBuilderNode(item.Data)}, nil
}
