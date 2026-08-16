package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type FinalizeAirGappedExportLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewFinalizeAirGappedExportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FinalizeAirGappedExportLogic {
	return &FinalizeAirGappedExportLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 绑定已复验的离线任务ZIP并开放结果导入。
func (l *FinalizeAirGappedExportLogic) FinalizeAirGappedExport(in *core.FinalizeAirGappedExportReq) (*core.AirGappedPackageResp, error) {
	return finalizeAirGappedExport(l.ctx, l.svcCtx, in)
}
