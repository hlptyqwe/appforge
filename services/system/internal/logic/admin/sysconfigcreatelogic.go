package adminlogic

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"
	"appforge/services/system/models"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SysConfigCreateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSysConfigCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysConfigCreateLogic {
	return &SysConfigCreateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SysConfigCreateLogic) SysConfigCreate(in *system.SysConfigCreateReq) (*system.RespBase, error) {
	if in == nil || strings.TrimSpace(in.GetConfigKey()) == "" {
		return nil, status.Error(codes.InvalidArgument, "config_key is required")
	}
	value := strings.TrimSpace(in.GetConfigValue())
	if value == "" {
		value = "{}"
	}
	if !json.Valid([]byte(value)) {
		return nil, status.Error(codes.InvalidArgument, "config_value must be valid JSON")
	}
	tenant, err := effectiveTenant(l.ctx, in.GetTenantId())
	if err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()
	item := &models.SysConfig{TenantId: tenant, ConfigKey: sql.NullString{String: strings.TrimSpace(in.GetConfigKey()), Valid: true}, ConfigValue: sql.NullString{String: value, Valid: true}, Remark: nullText(in.GetRemark()), CreateTimes: now, UpdateTimes: now}
	if _, err := l.svcCtx.ConfigModel.Insert(l.ctx, item); err != nil {
		return nil, status.Errorf(codes.Internal, "create config failed: %v", err)
	}
	return &system.RespBase{Base: responseBase()}, nil
}
