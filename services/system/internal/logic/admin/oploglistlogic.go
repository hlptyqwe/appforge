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

type OpLogListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewOpLogListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpLogListLogic {
	return &OpLogListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *OpLogListLogic) OpLogList(in *system.OpLogListReq) (*system.OpLogListResp, error) {
	if in == nil {
		in = &system.OpLogListReq{}
	}
	tenant, err := effectiveTenant(l.ctx, 0)
	if err != nil {
		return nil, err
	}
	cursor, limit := pageValues(in.GetPage())
	where := []string{"tenant_id = ?"}
	args := []any{tenant}
	if username := strings.TrimSpace(in.GetUsername()); username != "" {
		where = append(where, "username LIKE ?")
		args = append(args, "%"+username+"%")
	}
	if method := strings.TrimSpace(methodValue(in.GetMethod())); method != "" {
		where = append(where, "method = ?")
		args = append(args, method)
	}
	if path := strings.TrimSpace(in.GetPath()); path != "" {
		where = append(where, "path LIKE ?")
		args = append(args, "%"+path+"%")
	}
	whereSQL := strings.Join(where, " AND ")
	var total int64
	if err := l.svcCtx.DB.QueryRowCtx(l.ctx, &total, fmt.Sprintf("SELECT COUNT(1) FROM sys_op_log WHERE %s", whereSQL), args...); err != nil {
		return nil, status.Errorf(codes.Internal, "count operation logs failed: %v", err)
	}
	queryArgs := append(append([]any{}, args...), cursor, limit+1)
	var rows []models.SysOpLog
	query := fmt.Sprintf("SELECT id, tenant_id, user_id, username, module, action, method, path, req, resp, ip, cost_ms, create_times, update_times FROM sys_op_log WHERE %s AND id > ? ORDER BY id ASC LIMIT ?", whereSQL)
	if err := l.svcCtx.DB.QueryRowsCtx(l.ctx, &rows, query, queryArgs...); err != nil {
		return nil, status.Errorf(codes.Internal, "list operation logs failed: %v", err)
	}
	hasNext := int64(len(rows)) > limit
	if hasNext {
		rows = rows[:limit]
	}
	data := make([]*system.OpLogItem, 0, len(rows))
	var nextCursor int64
	for i := range rows {
		row := &rows[i]
		data = append(data, &system.OpLogItem{
			Id: row.Id, TenantId: row.TenantId, UserId: row.UserId, Username: row.Username,
			Module: row.Module, Action: row.Action, Method: requestMethod(row.Method), Path: row.Path,
			Req: row.Req.String, Resp: row.Resp.String, Ip: row.Ip, CostMs: row.CostMs,
			CreateTimes: row.CreateTimes, UpdateTimes: row.UpdateTimes,
		})
		nextCursor = row.Id
	}
	return &system.OpLogListResp{Base: responsePage(total, hasNext, nextCursor, cursor > 0, 0), Data: data}, nil
}

func methodValue(value system.RequestMethod) string {
	switch value {
	case system.RequestMethod_REQUEST_METHOD_GET:
		return "GET"
	case system.RequestMethod_REQUEST_METHOD_POST:
		return "POST"
	case system.RequestMethod_REQUEST_METHOD_PUT:
		return "PUT"
	case system.RequestMethod_REQUEST_METHOD_DELETE:
		return "DELETE"
	default:
		return ""
	}
}
