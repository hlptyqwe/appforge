package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type RecordOpenApiAuditLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRecordOpenApiAuditLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RecordOpenApiAuditLogic {
	return &RecordOpenApiAuditLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 写入Open API调用审计。
func (l *RecordOpenApiAuditLogic) RecordOpenApiAudit(in *core.RecordOpenApiAuditReq) (*core.RespBase, error) {
	return recordOpenApiAudit(l.ctx, l.svcCtx, in)
}
