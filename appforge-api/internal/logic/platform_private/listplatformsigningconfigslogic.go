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

type ListPlatformSigningConfigsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListPlatformSigningConfigsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPlatformSigningConfigsLogic {
	return &ListPlatformSigningConfigsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListPlatformSigningConfigsLogic) ListPlatformSigningConfigs(req *types.ListPlatformSigningConfigsReq) (resp *types.PlatformSigningConfigListResp, err error) {
	item, err := l.svcCtx.CoreCli.ListSigningConfigs(l.ctx, &corepb.SigningConfigListReq{
		Page: platformlogic.PlatformPage(req.PageReq), AppId: req.AppId, Status: req.Status,
	})
	if err != nil {
		return nil, err
	}
	data := make([]types.PlatformSigningConfig, 0, len(item.Data))
	for _, value := range item.Data {
		data = append(data, platformlogic.MapPlatformSigningConfig(value))
	}
	return &types.PlatformSigningConfigListResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: data}, nil
}
