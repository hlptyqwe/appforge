package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListInvoicesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListInvoicesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListInvoicesLogic {
	return &ListInvoicesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 查询租户不可变账单及账单项。
func (l *ListInvoicesLogic) ListInvoices(in *core.InvoiceListReq) (*core.InvoiceListResp, error) {
	return listInvoices(l.ctx, l.svcCtx, in)
}
