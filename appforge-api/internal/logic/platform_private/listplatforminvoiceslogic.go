// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/logicutil"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListPlatformInvoicesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListPlatformInvoicesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPlatformInvoicesLogic {
	return &ListPlatformInvoicesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListPlatformInvoicesLogic) ListPlatformInvoices(req *types.ListPlatformInvoicesReq) (resp *types.PlatformInvoiceListResp, err error) {
	return logicutil.Proxy[types.PlatformInvoiceListResp](l.ctx, req, l.svcCtx.CoreCli.ListInvoices)
}
