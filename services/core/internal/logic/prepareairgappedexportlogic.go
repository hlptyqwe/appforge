package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type PrepareAirGappedExportLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPrepareAirGappedExportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PrepareAirGappedExportLogic {
	return &PrepareAirGappedExportLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 管理端锁定指定任务并准备AIR_GAPPED离线任务包。
func (l *PrepareAirGappedExportLogic) PrepareAirGappedExport(in *core.PrepareAirGappedExportReq) (*core.PrepareAirGappedExportResp, error) {
	return prepareAirGappedExport(l.ctx, l.svcCtx, in)
}
