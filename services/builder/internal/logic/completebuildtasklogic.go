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

func (l *CompleteBuildTaskLogic) CompleteBuildTask(in *builder.CompleteBuildTaskReq) (*builder.RespBase, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	client, err := coreClient(l.svcCtx)
	if err != nil {
		return nil, err
	}
	resp, err := client.CompleteBuildTask(toCoreContext(l.ctx), &core.CompleteBuildTaskReq{TaskId: in.TaskId, BuilderId: in.BuilderId, ApkUrl: in.ApkUrl, ApkSha256: in.ApkSha256, ApkSize: in.ApkSize, LogUrl: in.LogUrl})
	if err != nil {
		return nil, err
	}
	return mapBase(resp), nil
}
