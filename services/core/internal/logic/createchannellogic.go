package logic

import (
	"context"
	"strings"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"
	"appforge/services/core/models"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreateChannelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateChannelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateChannelLogic {
	return &CreateChannelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateChannelLogic) CreateChannel(in *core.CreateChannelReq) (*core.ChannelResp, error) {
	tenant, err := tenantID(l.ctx)
	if err != nil {
		return nil, err
	}
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := requirePositive(in.AppId, "app_id"); err != nil {
		return nil, err
	}
	if err := requireText(in.ChannelName, "channel_name", 100); err != nil {
		return nil, err
	}
	app, err := l.svcCtx.ApplicationModel.FindOne(l.ctx, in.AppId)
	if err != nil {
		return nil, notFoundOrInternal(err, "application")
	}
	if err := ensureTenant(app.TenantId, tenant); err != nil {
		return nil, err
	}
	code := strings.TrimSpace(in.ChannelCode)
	if code == "" {
		code = newChannelCode()
	} else if err := requireText(code, "channel_code", 32); err != nil {
		return nil, err
	}
	if existing, findErr := l.svcCtx.ChannelModel.FindOneByChannelCode(l.ctx, code); findErr == nil && existing != nil {
		return nil, status.Error(codes.AlreadyExists, "channel_code already exists")
	} else if findErr != nil && findErr != models.ErrNotFound {
		return nil, status.Error(codes.Internal, "check channel_code failed")
	}
	if existing, findErr := l.svcCtx.ChannelModel.FindOneByTenantIdAppIdChannelName(l.ctx, tenant, in.AppId, strings.TrimSpace(in.ChannelName)); findErr == nil && existing != nil {
		return nil, status.Error(codes.AlreadyExists, "channel_name already exists")
	} else if findErr != nil && findErr != models.ErrNotFound {
		return nil, status.Error(codes.Internal, "check channel_name failed")
	}

	_, err = l.svcCtx.ChannelModel.Insert(l.ctx, &models.TPromotionChannel{
		TenantId:    tenant,
		AppId:       in.AppId,
		ChannelCode: code,
		ChannelName: strings.TrimSpace(in.ChannelName),
		LandingUrl:  nullString(in.LandingUrl),
		DownloadUrl: nullString(in.DownloadUrl),
		Status:      int64(core.ChannelStatus_CHANNEL_STATUS_ENABLED),
		CreateBy:    actorID(l.ctx),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create channel failed: %v", err)
	}
	item, err := l.svcCtx.ChannelModel.FindOneByChannelCode(l.ctx, code)
	if err != nil {
		return nil, notFoundOrInternal(err, "channel")
	}

	return &core.ChannelResp{Base: okBase(), Data: mapChannel(item)}, nil
}
