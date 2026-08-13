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

type ReportInstallLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewReportInstallLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReportInstallLogic {
	return &ReportInstallLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ReportInstallLogic) ReportInstall(in *core.InstallReportReq) (*core.RespBase, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := requirePositive(in.AppId, "app_id"); err != nil {
		return nil, err
	}
	if err := requireText(in.ChannelCode, "channel_code", 32); err != nil {
		return nil, err
	}
	if err := requireText(in.InstallId, "install_id", 128); err != nil {
		return nil, err
	}
	channel, err := l.svcCtx.ChannelModel.FindOneByChannelCode(l.ctx, strings.TrimSpace(in.ChannelCode))
	if err != nil {
		return nil, notFoundOrInternal(err, "channel")
	}
	if channel.AppId != in.AppId || channel.Status != int64(core.ChannelStatus_CHANNEL_STATUS_ENABLED) {
		return nil, status.Error(codes.InvalidArgument, "channel is invalid")
	}
	installID := strings.TrimSpace(in.InstallId)
	if existing, findErr := l.svcCtx.InstallModel.FindOneByInstallId(l.ctx, installID); findErr == nil {
		if existing.AppId != in.AppId || existing.ChannelId != channel.Id {
			return nil, status.Error(codes.AlreadyExists, "install_id is already bound to another channel")
		}
		return &core.RespBase{Base: okBase()}, nil
	} else if findErr != models.ErrNotFound {
		return nil, status.Errorf(codes.Internal, "check install failed: %v", findErr)
	}
	firstOpen := timeFromMillis(in.FirstOpenTime)
	_, err = l.svcCtx.InstallModel.Insert(l.ctx, &models.TChannelInstall{
		TenantId:      channel.TenantId,
		AppId:         channel.AppId,
		ChannelId:     channel.Id,
		ChannelCode:   channel.ChannelCode,
		InstallId:     installID,
		AppVersion:    nullString(in.AppVersion),
		DeviceModel:   nullString(in.DeviceModel),
		Ip:            nullString(in.Ip),
		FirstOpenTime: firstOpen,
	})
	if err != nil {
		if existing, findErr := l.svcCtx.InstallModel.FindOneByInstallId(l.ctx, installID); findErr == nil && existing.AppId == in.AppId && existing.ChannelId == channel.Id {
			return &core.RespBase{Base: okBase()}, nil
		}
		return nil, status.Errorf(codes.Internal, "record install failed: %v", err)
	}
	if _, findErr := l.svcCtx.ChannelEventModel.FindOneByTenantIdEventTypeEventKey(l.ctx, channel.TenantId, eventTypeInstall, installID); findErr == models.ErrNotFound {
		_, _ = l.svcCtx.ChannelEventModel.Insert(l.ctx, &models.TChannelEvent{
			TenantId:    channel.TenantId,
			AppId:       channel.AppId,
			ChannelId:   channel.Id,
			ChannelCode: channel.ChannelCode,
			EventType:   eventTypeInstall,
			EventKey:    installID,
			InstallId:   nullString(installID),
			AppVersion:  nullString(in.AppVersion),
			Ip:          nullString(in.Ip),
			DeviceModel: nullString(in.DeviceModel),
			EventTime:   firstOpen,
		})
	}

	return &core.RespBase{Base: okBase()}, nil
}
