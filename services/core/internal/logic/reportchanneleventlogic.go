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

type ReportChannelEventLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewReportChannelEventLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReportChannelEventLogic {
	return &ReportChannelEventLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ReportChannelEventLogic) ReportChannelEvent(in *core.ReportChannelEventReq) (*core.RespBase, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := requireText(in.ChannelCode, "channel_code", 64); err != nil {
		return nil, err
	}
	if err := requireText(in.EventKey, "event_key", 128); err != nil {
		return nil, err
	}
	if in.EventType != eventTypeRegister && in.EventType != eventTypeFirstPay && in.EventType != eventTypePay {
		return nil, status.Error(codes.InvalidArgument, "unsupported channel event type")
	}
	for _, check := range []struct {
		value string
		field string
		max   int
	}{{in.InstallId, "install_id", 128}, {in.AppVersion, "app_version", 64}, {in.Ip, "ip", 64}, {in.DeviceModel, "device_model", 128}, {in.Metadata, "metadata", 2000}} {
		if err := requireOptionalText(check.value, check.field, check.max); err != nil {
			return nil, err
		}
	}
	if (in.EventType == eventTypeRegister || in.EventType == eventTypeFirstPay) && strings.TrimSpace(in.InstallId) == "" {
		return nil, status.Error(codes.InvalidArgument, "install_id is required for this event type")
	}
	channel, err := l.svcCtx.ChannelModel.FindOneByChannelCode(l.ctx, strings.TrimSpace(in.ChannelCode))
	if err != nil {
		return nil, notFoundOrInternal(err, "channel")
	}
	if in.AppId > 0 && in.AppId != channel.AppId {
		return nil, status.Error(codes.NotFound, "channel not found")
	}
	if existing, findErr := l.svcCtx.ChannelEventModel.FindOneByTenantIdEventTypeEventKey(l.ctx, channel.TenantId, int64(in.EventType), strings.TrimSpace(in.EventKey)); findErr == nil && existing != nil {
		return &core.RespBase{Base: okBase()}, nil
	} else if findErr != nil && findErr != models.ErrNotFound {
		return nil, status.Errorf(codes.Internal, "check channel event failed: %v", findErr)
	}
	if installID := strings.TrimSpace(in.InstallId); installID != "" {
		install, findErr := l.svcCtx.InstallModel.FindOneByInstallId(l.ctx, installID)
		if findErr != nil || install.AppId != channel.AppId || install.ChannelId != channel.Id {
			return nil, status.Error(codes.InvalidArgument, "install_id is not bound to this channel")
		}
		eventTime := timeFromMillis(in.EventTime)
		if in.EventType == eventTypeRegister && !install.RegisterTime.Valid {
			install.RegisterUserId = nullInt64(in.UserId)
			install.RegisterTime = nullTime(eventTime)
			if err := l.svcCtx.InstallModel.Update(l.ctx, install); err != nil {
				return nil, status.Errorf(codes.Internal, "update registration attribution failed: %v", err)
			}
		}
		if in.EventType == eventTypeFirstPay && !install.FirstPayTime.Valid {
			install.FirstPayTime = nullTime(eventTime)
			if err := l.svcCtx.InstallModel.Update(l.ctx, install); err != nil {
				return nil, status.Errorf(codes.Internal, "update first payment attribution failed: %v", err)
			}
		}
	}
	_, err = l.svcCtx.ChannelEventModel.Insert(l.ctx, &models.TChannelEvent{
		TenantId: channel.TenantId, AppId: channel.AppId, ChannelId: channel.Id, ChannelCode: channel.ChannelCode,
		EventType: int64(in.EventType), EventKey: strings.TrimSpace(in.EventKey), InstallId: nullString(in.InstallId),
		UserId: nullInt64(in.UserId), AppVersion: nullString(in.AppVersion), Ip: nullString(in.Ip),
		DeviceModel: nullString(in.DeviceModel), EventTime: timeFromMillis(in.EventTime), Metadata: nullString(in.Metadata),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "record channel event failed: %v", err)
	}

	return &core.RespBase{Base: okBase()}, nil
}
