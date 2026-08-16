package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type AbortAirGappedExportLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAbortAirGappedExportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AbortAirGappedExportLogic {
	return &AbortAirGappedExportLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 撤销离线包并以失败状态关闭当前任务attempt。
func (l *AbortAirGappedExportLogic) AbortAirGappedExport(in *core.AbortAirGappedExportReq) (*core.AirGappedPackageResp, error) {
	return abortAirGappedExport(l.ctx, l.svcCtx, in)
}
