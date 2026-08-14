package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ConfirmQuotaLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewConfirmQuotaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfirmQuotaLogic {
	return &ConfirmQuotaLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 幂等确认额度预占并写不可变用量账本。
func (l *ConfirmQuotaLogic) ConfirmQuota(in *core.QuotaReservationActionReq) (*core.QuotaReservationResp, error) {
	return confirmQuota(l.ctx, l.svcCtx, in)
}
