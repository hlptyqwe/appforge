package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type BeginOpenApiIdempotencyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBeginOpenApiIdempotencyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BeginOpenApiIdempotencyLogic {
	return &BeginOpenApiIdempotencyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 申请或读取Open API幂等执行记录。
func (l *BeginOpenApiIdempotencyLogic) BeginOpenApiIdempotency(in *core.BeginOpenApiIdempotencyReq) (*core.OpenApiIdempotencyResp, error) {
	return beginOpenApiIdempotency(l.ctx, l.svcCtx, in)
}
