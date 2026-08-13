package adminlogic

import (
	"context"
	"fmt"
	"strings"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"
	"appforge/services/system/models"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SysConfigListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSysConfigListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysConfigListLogic {
	return &SysConfigListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SysConfigListLogic) SysConfigList(in *system.SysConfigListReq) (*system.SysConfigListResp, error) {
	if in == nil {
		in = &system.SysConfigListReq{}
	}
	tenant, err := effectiveTenant(l.ctx, in.GetTenantId())
	if err != nil {
		return nil, err
	}
	cursor, limit := pageValues(in.GetPage())
	where := []string{"tenant_id = ?"}
	args := []any{tenant}
	if keyword := strings.TrimSpace(in.GetKeyword()); keyword != "" {
		where = append(where, "(config_key LIKE ? OR remark LIKE ?)")
		like := "%" + keyword + "%"
		args = append(args, like, like)
	}
	whereSQL := strings.Join(where, " AND ")
	var total int64
	if err := l.svcCtx.DB.QueryRowCtx(l.ctx, &total, fmt.Sprintf("SELECT COUNT(1) FROM sys_config WHERE %s", whereSQL), args...); err != nil {
		return nil, status.Errorf(codes.Internal, "count configs failed: %v", err)
	}
	queryArgs := append(append([]any{}, args...), cursor, limit+1)
	var rows []models.SysConfig
	query := fmt.Sprintf("SELECT id, tenant_id, config_key, config_value, remark, create_times, update_times FROM sys_config WHERE %s AND id > ? ORDER BY id ASC LIMIT ?", whereSQL)
	if err := l.svcCtx.DB.QueryRowsCtx(l.ctx, &rows, query, queryArgs...); err != nil {
		return nil, status.Errorf(codes.Internal, "list configs failed: %v", err)
	}
	hasNext := int64(len(rows)) > limit
	if hasNext {
		rows = rows[:limit]
	}
	data := make([]*system.SysConfigItem, 0, len(rows))
	var nextCursor int64
	for i := range rows {
		data = append(data, configItem(&rows[i]))
		nextCursor = rows[i].Id
	}
	return &system.SysConfigListResp{Base: responsePage(total, hasNext, nextCursor, cursor > 0, 0), Data: data}, nil
}
