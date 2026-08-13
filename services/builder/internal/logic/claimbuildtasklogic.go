package logic

import (
	"context"

	"appforge/proto/builder"
	corepb "appforge/proto/core"
	"appforge/services/builder/internal/svc"

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
	return &ClaimBuildTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ClaimBuildTaskLogic) ClaimBuildTask(in *builder.ClaimBuildTaskReq) (*builder.BuildTaskResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	client, err := coreClient(l.svcCtx)
	if err != nil {
		return nil, err
	}
	resp, err := client.ClaimBuildTask(toCoreContext(l.ctx), &corepb.ClaimBuildTaskReq{BuilderId: in.BuilderId, LeaseSeconds: in.LeaseSeconds})
	if err != nil {
		return nil, err
	}
	return &builder.BuildTaskResp{Base: resp.Base, Data: mapBuildTask(resp.Data)}, nil
}
