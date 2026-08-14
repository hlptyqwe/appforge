// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RevokePlatformSourceRepositoryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRevokePlatformSourceRepositoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RevokePlatformSourceRepositoryLogic {
	return &RevokePlatformSourceRepositoryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RevokePlatformSourceRepositoryLogic) RevokePlatformSourceRepository(req *types.PlatformIdReq) (resp *types.PlatformSourceRepositoryResp, err error) {
	return revokePlatformSourceRepository(l.ctx, l.svcCtx, req)
}
