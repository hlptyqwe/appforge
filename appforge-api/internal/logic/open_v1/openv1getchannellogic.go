// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package open_v1

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpenV1GetChannelLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpenV1GetChannelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenV1GetChannelLogic {
	return &OpenV1GetChannelLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpenV1GetChannelLogic) OpenV1GetChannel(req *types.PlatformIdReq) (resp *types.PlatformChannelResp, err error) {
	return openV1GetChannel(l.ctx, l.svcCtx, req)
}
