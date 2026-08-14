package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetBillingUsageLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetBillingUsageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetBillingUsageLogic {
	return &GetBillingUsageLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 查询周期用量、预占和当前限额。
func (l *GetBillingUsageLogic) GetBillingUsage(in *core.BillingUsageReq) (*core.BillingUsageResp, error) {
	return getBillingUsage(l.ctx, l.svcCtx, in)
}
