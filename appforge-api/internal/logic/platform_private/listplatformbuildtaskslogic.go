// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	platformlogic "appforge/admin-api/internal/logic"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	corepb "appforge/proto/core"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListPlatformBuildTasksLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListPlatformBuildTasksLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPlatformBuildTasksLogic {
	return &ListPlatformBuildTasksLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListPlatformBuildTasksLogic) ListPlatformBuildTasks(req *types.ListPlatformBuildTasksReq) (resp *types.PlatformBuildTaskListResp, err error) {
	item, err := l.svcCtx.CoreCli.ListBuildTasks(l.ctx, &corepb.BuildTaskListReq{Page: platformlogic.PlatformPage(req.PageReq), AppId: req.AppId, ChannelId: req.ChannelId, Status: corepb.BuildTaskStatus(req.Status)})
	if err != nil {
		return nil, err
	}
	data := make([]types.PlatformBuildTask, 0, len(item.Data))
	for _, value := range item.Data {
		data = append(data, platformlogic.MapPlatformBuildTask(value))
	}
	return &types.PlatformBuildTaskListResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: data}, nil
}
