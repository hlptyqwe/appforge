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

type FailBuildTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewFailBuildTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FailBuildTaskLogic {
	return &FailBuildTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *FailBuildTaskLogic) FailBuildTask(in *builder.FailBuildTaskReq) (*builder.RespBase, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	client, err := coreClient(l.svcCtx)
	if err != nil {
		return nil, err
	}
	resp, err := client.FailBuildTask(toCoreContext(l.ctx), &core.FailBuildTaskReq{
		TaskId: in.TaskId, BuilderId: in.BuilderId, ErrorMessage: in.ErrorMessage, LogUrl: in.LogUrl,
		LogObjectKey: in.LogObjectKey, LogSha256: in.LogSha256, LogSize: in.LogSize, BuilderAttempt: in.BuilderAttempt,
	})
	if err != nil {
		return nil, err
	}
	return mapBase(resp), nil
}
