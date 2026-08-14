package adminlogic

import (
	"context"

	"appforge/common/utils"
	"appforge/proto/system"
	"appforge/services/system/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SysPermListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSysPermListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysPermListLogic {
	return &SysPermListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SysPermListLogic) SysPermList(in *system.Empty) (*system.SysPermListResp, error) {
	type permissionRow struct {
		PermKey  string `db:"perms"`
		Method   string `db:"method"`
		Path     string `db:"path"`
		Name     string `db:"name"`
		AppScope int64  `db:"app_scope"`
	}

	var rows []permissionRow
	query := `
		SELECT perms, method, path, name, app_scope
		FROM sys_menu
		WHERE menu_type = 3 AND enabled = 1
		  AND perms <> '' AND method <> '' AND path <> ''`
	args := make([]any, 0, 1)
	if scope, scopeErr := utils.GetAppScopeFromMd(l.ctx); scopeErr == nil && scope > 0 {
		query += " AND app_scope = ?"
		args = append(args, scope)
	}
	query += " ORDER BY app_scope, path, method, id"
	if err := l.svcCtx.DB.QueryRowsCtx(l.ctx, &rows, query, args...); err != nil {
		return nil, status.Errorf(codes.Internal, "query permission routes failed: %v", err)
	}

	items := make([]*system.SysPermItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, &system.SysPermItem{
			PermKey:  row.PermKey,
			Method:   requestMethod(row.Method),
			Path:     row.Path,
			Name:     row.Name,
			AppScope: system.ApplicationScope(row.AppScope),
		})
	}

	return &system.SysPermListResp{Base: responseBase(), Data: items}, nil
}
