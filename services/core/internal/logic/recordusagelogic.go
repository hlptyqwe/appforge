package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type RecordUsageLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRecordUsageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RecordUsageLogic {
	return &RecordUsageLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 追加一条幂等不可变用量记录。
func (l *RecordUsageLogic) RecordUsage(in *core.RecordUsageReq) (*core.RespBase, error) {
	return recordUsage(l.ctx, l.svcCtx, in)
}
