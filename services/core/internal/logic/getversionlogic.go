package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetVersionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetVersionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetVersionLogic {
	return &GetVersionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetVersionLogic) GetVersion(in *core.VersionIdReq) (*core.VersionResp, error) {
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
	item, err := l.svcCtx.VersionModel.FindOne(l.ctx, in.Id)
	if err != nil {
		return nil, notFoundOrInternal(err, "version")
	}
	if err := ensureTenant(item.TenantId, tenant); err != nil {
		return nil, err
	}

	return &core.VersionResp{Base: okBase(), Data: mapVersion(item)}, nil
}
