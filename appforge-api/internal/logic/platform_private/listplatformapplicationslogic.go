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

type ListPlatformApplicationsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListPlatformApplicationsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPlatformApplicationsLogic {
	return &ListPlatformApplicationsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListPlatformApplicationsLogic) ListPlatformApplications(req *types.ListPlatformApplicationsReq) (resp *types.PlatformApplicationListResp, err error) {
	item, err := l.svcCtx.CoreCli.ListApplications(l.ctx, &core.ApplicationListReq{Page: platformlogic.PlatformPage(req.PageReq), Keyword: req.Keyword, Status: core.ApplicationStatus(req.Status)})
	if err != nil {
		return nil, err
	}
	data := make([]types.PlatformApplication, 0, len(item.Data))
	for _, value := range item.Data {
		data = append(data, platformlogic.MapPlatformApplication(value))
	}
	return &types.PlatformApplicationListResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: data}, nil
}
