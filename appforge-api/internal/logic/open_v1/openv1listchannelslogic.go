// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package open_v1

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpenV1ListChannelsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpenV1ListChannelsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenV1ListChannelsLogic {
	return &OpenV1ListChannelsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpenV1ListChannelsLogic) OpenV1ListChannels(req *types.ListPlatformChannelsReq) (resp *types.PlatformChannelListResp, err error) {
	return openV1ListChannels(l.ctx, l.svcCtx, req)
}
