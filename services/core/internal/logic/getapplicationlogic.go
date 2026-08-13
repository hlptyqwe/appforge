package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetApplicationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetApplicationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetApplicationLogic {
	return &GetApplicationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetApplicationLogic) GetApplication(in *core.ApplicationIdReq) (*core.ApplicationResp, error) {
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
	item, err := l.svcCtx.ApplicationModel.FindOne(l.ctx, in.Id)
	if err != nil {
		return nil, notFoundOrInternal(err, "application")
	}
	if err := ensureTenant(item.TenantId, tenant); err != nil {
		return nil, err
	}

	return &core.ApplicationResp{Base: okBase(), Data: mapApplication(item)}, nil
}
