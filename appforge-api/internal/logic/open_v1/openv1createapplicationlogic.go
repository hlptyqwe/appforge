// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package open_v1

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpenV1CreateApplicationLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpenV1CreateApplicationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenV1CreateApplicationLogic {
	return &OpenV1CreateApplicationLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpenV1CreateApplicationLogic) OpenV1CreateApplication(req *types.CreatePlatformApplicationReq) (resp *types.PlatformApplicationResp, err error) {
	return openV1CreateApplication(l.ctx, l.svcCtx, req)
}
