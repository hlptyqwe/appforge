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

type SysMenuListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSysMenuListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysMenuListLogic {
	return &SysMenuListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SysMenuListLogic) SysMenuList(in *system.SysMenuListReq) (*system.SysMenuListResp, error) {
	if in == nil {
		in = &system.SysMenuListReq{}
	}
	tenant, err := effectiveTenant(l.ctx, 0)
	if err != nil {
		return nil, err
	}
	_ = tenant
	appScope := effectiveAppScope(l.ctx, in.GetAppScope())
	cursor, limit := pageValues(in.GetPage())
	where := []string{"app_scope = ?"}
	args := []any{appScope}
	if keyword := strings.TrimSpace(in.GetKeyword()); keyword != "" {
		where = append(where, "(name LIKE ? OR perms LIKE ? OR path LIKE ?)")
		like := "%" + keyword + "%"
		args = append(args, like, like, like)
	}
	if value := int64(in.GetMenuType()); value > 0 {
		where = append(where, "menu_type = ?")
		args = append(args, value)
	}
	if value := int64(in.GetEnabled()); value > 0 {
		where = append(where, "enabled = ?")
		args = append(args, value)
	}
	if value := int64(in.GetVisible()); value > 0 {
		where = append(where, "visible = ?")
		args = append(args, value)
	}
	whereSQL := strings.Join(where, " AND ")
	var total int64
	if err := l.svcCtx.DB.QueryRowCtx(l.ctx, &total, fmt.Sprintf("SELECT COUNT(1) FROM sys_menu WHERE %s", whereSQL), args...); err != nil {
		return nil, status.Errorf(codes.Internal, "count menus failed: %v", err)
	}
	queryArgs := append(append([]any{}, args...), cursor, limit+1)
	var rows []models.SysMenu
	query := fmt.Sprintf("SELECT id, parent_id, app_scope, name, menu_type, method, path, component, perms, icon, sort, visible, enabled, create_times, update_times FROM sys_menu WHERE %s AND id > ? ORDER BY sort ASC, id ASC LIMIT ?", whereSQL)
	if err := l.svcCtx.DB.QueryRowsCtx(l.ctx, &rows, query, queryArgs...); err != nil {
		return nil, status.Errorf(codes.Internal, "list menus failed: %v", err)
	}
	hasNext := int64(len(rows)) > limit
	if hasNext {
		rows = rows[:limit]
	}
	data := make([]*system.SysMenuItem, 0, len(rows))
	var nextCursor int64
	for i := range rows {
		data = append(data, menuItem(&rows[i]))
		nextCursor = rows[i].Id
	}
	return &system.SysMenuListResp{
		Base: responsePage(total, hasNext, nextCursor, cursor > 0, 0), Data: data,
	}, nil
}
