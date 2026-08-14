package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CompleteOpenApiIdempotencyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCompleteOpenApiIdempotencyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CompleteOpenApiIdempotencyLogic {
	return &CompleteOpenApiIdempotencyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 保存Open API幂等请求的最终响应。
func (l *CompleteOpenApiIdempotencyLogic) CompleteOpenApiIdempotency(in *core.CompleteOpenApiIdempotencyReq) (*core.RespBase, error) {
	return completeOpenApiIdempotency(l.ctx, l.svcCtx, in)
}
