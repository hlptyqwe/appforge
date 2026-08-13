package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ClaimBuildTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewClaimBuildTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClaimBuildTaskLogic {
	return &ClaimBuildTaskLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ClaimBuildTaskLogic) ClaimBuildTask(in *core.ClaimBuildTaskReq) (*core.BuildTaskResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := validateBuilderRequest(in.BuilderId); err != nil {
		return nil, err
	}
	item, err := claimTask(l.ctx, l.svcCtx, in.BuilderId, leaseSeconds(in.LeaseSeconds))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "claim build task failed: %v", err)
	}
	if item == nil {
		return &core.BuildTaskResp{Base: okBase()}, nil
	}
	return &core.BuildTaskResp{Base: okBase(), Data: mapBuildTask(item)}, nil
}
