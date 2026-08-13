package logic

import (
	"context"
	"strings"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CompleteBuildTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCompleteBuildTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CompleteBuildTaskLogic {
	return &CompleteBuildTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CompleteBuildTaskLogic) CompleteBuildTask(in *core.CompleteBuildTaskReq) (*core.RespBase, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := requireText(in.ApkUrl, "apk_url", 500); err != nil {
		return nil, err
	}
	if sha := strings.TrimSpace(in.ApkSha256); sha != "" && len(sha) != 64 {
		return nil, status.Error(codes.InvalidArgument, "apk_sha256 must be 64 characters")
	}
	if in.ApkSize < 0 {
		return nil, status.Error(codes.InvalidArgument, "apk_size must not be negative")
	}
	if err := updateTaskWithBuilder(l.ctx, l.svcCtx, in.TaskId, in.BuilderId,
		`UPDATE t_build_task SET status = ?, apk_url = ?, apk_sha256 = NULLIF(?, ''), apk_size = ?, log_url = NULLIF(?, ''), error_message = NULL, finish_time = CURRENT_TIMESTAMP, lease_until = NULL, update_time = CURRENT_TIMESTAMP WHERE status IN (?, ?, ?) AND id = ? AND builder_id = ?`,
		buildStatusSuccess, in.ApkUrl, in.ApkSha256, in.ApkSize, in.LogUrl, buildStatusBuilding, buildStatusSigning, buildStatusUploading); err != nil {
		return nil, err
	}

	return workerBase(), nil
}
