// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type TestPlatformWebhookEndpointLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTestPlatformWebhookEndpointLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TestPlatformWebhookEndpointLogic {
	return &TestPlatformWebhookEndpointLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TestPlatformWebhookEndpointLogic) TestPlatformWebhookEndpoint(req *types.PlatformIdReq) (resp *types.RespBase, err error) {
	return testPlatformWebhookEndpoint(l.ctx, l.svcCtx, req)
}
