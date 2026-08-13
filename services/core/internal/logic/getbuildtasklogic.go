package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetBuildTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetBuildTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetBuildTaskLogic {
	return &GetBuildTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetBuildTaskLogic) GetBuildTask(in *core.BuildTaskIdReq) (*core.BuildTaskResp, error) {
	tenant, err := tenantID(l.ctx)
	if err != nil {
		return nil, err
	}
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := requirePositive(in.Id, "id"); err != nil {
		return nil, err
	}
	item, err := l.svcCtx.BuildTaskModel.FindOne(l.ctx, in.Id)
	if err != nil {
		return nil, notFoundOrInternal(err, "build task")
	}
	if err := ensureTenant(item.TenantId, tenant); err != nil {
		return nil, err
	}

	return &core.BuildTaskResp{Base: okBase(), Data: mapBuildTask(item)}, nil
}
