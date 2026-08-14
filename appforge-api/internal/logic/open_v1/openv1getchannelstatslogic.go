// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package open_v1

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpenV1GetChannelStatsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpenV1GetChannelStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenV1GetChannelStatsLogic {
	return &OpenV1GetChannelStatsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpenV1GetChannelStatsLogic) OpenV1GetChannelStats(req *types.GetPlatformChannelStatsReq) (resp *types.PlatformChannelStatsResp, err error) {
	return openV1GetChannelStats(l.ctx, l.svcCtx, req)
}
