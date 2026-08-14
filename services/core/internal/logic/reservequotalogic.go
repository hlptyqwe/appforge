package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ReserveQuotaLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewReserveQuotaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReserveQuotaLogic {
	return &ReserveQuotaLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 并发安全地预占构建、存储或席位额度。
func (l *ReserveQuotaLogic) ReserveQuota(in *core.ReserveQuotaReq) (*core.QuotaReservationResp, error) {
	return reserveQuota(l.ctx, l.svcCtx, in)
}
