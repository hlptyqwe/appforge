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

type ListPlatformVersionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListPlatformVersionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPlatformVersionsLogic {
	return &ListPlatformVersionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListPlatformVersionsLogic) ListPlatformVersions(req *types.ListPlatformVersionsReq) (resp *types.PlatformVersionListResp, err error) {
	item, err := l.svcCtx.CoreCli.ListVersions(l.ctx, &corepb.VersionListReq{Page: platformlogic.PlatformPage(req.PageReq), AppId: req.AppId, Status: corepb.VersionStatus(req.Status)})
	if err != nil {
		return nil, err
	}
	data := make([]types.PlatformVersion, 0, len(item.Data))
	for _, value := range item.Data {
		data = append(data, platformlogic.MapPlatformVersion(value))
	}
	return &types.PlatformVersionListResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: data}, nil
}
