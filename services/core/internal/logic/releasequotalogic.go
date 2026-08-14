package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ReleaseQuotaLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewReleaseQuotaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReleaseQuotaLogic {
	return &ReleaseQuotaLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 幂等释放未使用额度预占。
func (l *ReleaseQuotaLogic) ReleaseQuota(in *core.QuotaReservationActionReq) (*core.QuotaReservationResp, error) {
	return releaseQuota(l.ctx, l.svcCtx, in)
}
