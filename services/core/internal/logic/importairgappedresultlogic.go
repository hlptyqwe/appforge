package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ImportAirGappedResultLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewImportAirGappedResultLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ImportAirGappedResultLogic {
	return &ImportAirGappedResultLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 校验Agent证书、签名、Nonce和attempt后导入离线结果。
func (l *ImportAirGappedResultLogic) ImportAirGappedResult(in *core.ImportAirGappedResultReq) (*core.AirGappedPackageResp, error) {
	return importAirGappedResult(l.ctx, l.svcCtx, in)
}
