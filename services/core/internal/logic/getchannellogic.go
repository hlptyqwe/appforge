package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetChannelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetChannelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetChannelLogic {
	return &GetChannelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetChannelLogic) GetChannel(in *core.ChannelIdReq) (*core.ChannelResp, error) {
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
	item, err := l.svcCtx.ChannelModel.FindOne(l.ctx, in.Id)
	if err != nil {
		return nil, notFoundOrInternal(err, "channel")
	}
	if err := ensureTenant(item.TenantId, tenant); err != nil {
		return nil, err
	}

	return &core.ChannelResp{Base: okBase(), Data: mapChannel(item)}, nil
}
