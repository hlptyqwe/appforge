package logic

import (
	"context"

	"appforge/proto/builder"
	"appforge/proto/core"
	"appforge/services/builder/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ReportBuildProgressLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewReportBuildProgressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReportBuildProgressLogic {
	return &ReportBuildProgressLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ReportBuildProgressLogic) ReportBuildProgress(in *builder.ReportBuildProgressReq) (*builder.RespBase, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	client, err := coreClient(l.svcCtx)
	if err != nil {
		return nil, err
	}
	resp, err := client.ReportBuildProgress(toCoreContext(l.ctx), &core.ReportBuildProgressReq{TaskId: in.TaskId, BuilderId: in.BuilderId, Status: core.BuildTaskStatus(in.Status), Message: in.Message, Progress: in.Progress, BuilderAttempt: in.BuilderAttempt})
	if err != nil {
		return nil, err
	}
	return mapBase(resp), nil
}
