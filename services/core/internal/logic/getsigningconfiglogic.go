package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetSigningConfigLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetSigningConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSigningConfigLogic {
	return &GetSigningConfigLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetSigningConfigLogic) GetSigningConfig(in *core.SigningConfigIdReq) (*core.SigningConfigResp, error) {
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
	item, err := l.svcCtx.SigningConfigModel.FindOne(l.ctx, in.Id)
	if err != nil {
		return nil, notFoundOrInternal(err, "signing config")
	}
	if err := ensureTenant(item.TenantId, tenant); err != nil {
		return nil, err
	}

	return &core.SigningConfigResp{Base: okBase(), Data: mapSigningConfig(item)}, nil
}
