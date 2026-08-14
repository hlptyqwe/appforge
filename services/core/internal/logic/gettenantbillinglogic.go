package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetTenantBillingLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetTenantBillingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTenantBillingLogic {
	return &GetTenantBillingLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 查询租户当前订阅、权益和固化套餐版本。
func (l *GetTenantBillingLogic) GetTenantBilling(in *core.TenantBillingReq) (*core.TenantBillingResp, error) {
	return getTenantBilling(l.ctx, l.svcCtx, in)
}
