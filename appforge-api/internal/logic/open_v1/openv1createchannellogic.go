// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package open_v1

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpenV1CreateChannelLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpenV1CreateChannelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenV1CreateChannelLogic {
	return &OpenV1CreateChannelLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpenV1CreateChannelLogic) OpenV1CreateChannel(req *types.CreatePlatformChannelReq) (resp *types.PlatformChannelResp, err error) {
	return openV1CreateChannel(l.ctx, l.svcCtx, req)
}
