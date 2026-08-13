package adminlogic

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SysConfigUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSysConfigUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysConfigUpdateLogic {
	return &SysConfigUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SysConfigUpdateLogic) SysConfigUpdate(in *system.SysConfigUpdateReq) (*system.RespBase, error) {
	if in == nil || in.GetId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	item, err := l.svcCtx.ConfigModel.FindOne(l.ctx, in.GetId())
	if err != nil {
		return nil, notFound(err, "system config")
	}
	if key := strings.TrimSpace(in.GetConfigKey()); key != "" {
		item.ConfigKey = sql.NullString{String: key, Valid: true}
	}
	if value := strings.TrimSpace(in.GetConfigValue()); value != "" {
		if !json.Valid([]byte(value)) {
			return nil, status.Error(codes.InvalidArgument, "config_value must be valid JSON")
		}
		item.ConfigValue = sql.NullString{String: value, Valid: true}
	}
	item.Remark = nullText(in.GetRemark())
	item.UpdateTimes = time.Now().UnixMilli()
	if err := l.svcCtx.ConfigModel.Update(l.ctx, item); err != nil {
		return nil, status.Errorf(codes.Internal, "update config failed: %v", err)
	}
	return &system.RespBase{Base: responseBase()}, nil
}
