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

type CreatePlatformBuildTaskLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreatePlatformBuildTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePlatformBuildTaskLogic {
	return &CreatePlatformBuildTaskLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreatePlatformBuildTaskLogic) CreatePlatformBuildTask(req *types.CreatePlatformBuildTaskReq) (resp *types.PlatformBuildTaskResp, err error) {
	item, err := l.svcCtx.CoreCli.CreateBuildTask(l.ctx, &core.CreateBuildTaskReq{AppId: req.AppId, VersionId: req.VersionId, ChannelId: req.ChannelId, SigningConfigId: req.SigningConfigId, Priority: req.Priority})
	if err != nil {
		return nil, err
	}
	return &types.PlatformBuildTaskResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: platformlogic.MapPlatformBuildTask(item.Data)}, nil
}
