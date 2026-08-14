package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ReportLocalAgentBuildProgressLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewReportLocalAgentBuildProgressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReportLocalAgentBuildProgressLogic {
	return &ReportLocalAgentBuildProgressLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Agent回报预定义构建阶段，不支持任意命令。
func (l *ReportLocalAgentBuildProgressLogic) ReportLocalAgentBuildProgress(in *core.ReportLocalAgentBuildProgressReq) (*core.RespBase, error) {
	return reportLocalAgentBuildProgress(l.ctx, l.svcCtx, in)
}
