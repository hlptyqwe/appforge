package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

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

func (l *ReportBuildProgressLogic) ReportBuildProgress(in *core.ReportBuildProgressReq) (*core.RespBase, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	statusValue, err := protoStatusToDB(in.Status)
	if err != nil {
		return nil, err
	}
	if statusValue != buildStatusBuilding && statusValue != buildStatusSigning && statusValue != buildStatusUploading {
		return nil, status.Error(codes.InvalidArgument, "progress status must be BUILDING, SIGNING or UPLOADING")
	}
	if in.Progress < 0 || in.Progress > 100 {
		return nil, status.Error(codes.InvalidArgument, "progress must be between 0 and 100")
	}
	if err := updateTaskWithBuilder(l.ctx, l.svcCtx, in.TaskId, in.BuilderId, in.BuilderAttempt,
		`UPDATE t_build_task SET status = ?, error_message = NULLIF(?, ''), update_time = CURRENT_TIMESTAMP WHERE status IN (?, ?, ?) AND id = ? AND builder_id = ? AND builder_attempt = ? AND lease_until > CURRENT_TIMESTAMP`,
		statusValue, in.Message, buildStatusBuilding, buildStatusSigning, buildStatusUploading); err != nil {
		return nil, err
	}

	return workerBase(), nil
}
