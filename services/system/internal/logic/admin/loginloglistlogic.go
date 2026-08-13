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

type LoginLogListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLoginLogListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogListLogic {
	return &LoginLogListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *LoginLogListLogic) LoginLogList(in *system.LoginLogListReq) (*system.LoginLogListResp, error) {
	if in == nil {
		in = &system.LoginLogListReq{}
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
	if in.GetSuccess() != 0 {
		where = append(where, "success = ?")
		args = append(args, in.GetSuccess())
	}
	whereSQL := strings.Join(where, " AND ")
	var total int64
	if err := l.svcCtx.DB.QueryRowCtx(l.ctx, &total, fmt.Sprintf("SELECT COUNT(1) FROM sys_login_log WHERE %s", whereSQL), args...); err != nil {
		return nil, status.Errorf(codes.Internal, "count login logs failed: %v", err)
	}
	queryArgs := append(append([]any{}, args...), cursor, limit+1)
	var rows []models.SysLoginLog
	query := fmt.Sprintf("SELECT id, tenant_id, user_id, username, ip, ua, success, msg, login_at FROM sys_login_log WHERE %s AND id > ? ORDER BY id ASC LIMIT ?", whereSQL)
	if err := l.svcCtx.DB.QueryRowsCtx(l.ctx, &rows, query, queryArgs...); err != nil {
		return nil, status.Errorf(codes.Internal, "list login logs failed: %v", err)
	}
	hasNext := int64(len(rows)) > limit
	if hasNext {
		rows = rows[:limit]
	}
	data := make([]*system.LoginLogItem, 0, len(rows))
	var nextCursor int64
	for i := range rows {
		row := &rows[i]
		data = append(data, &system.LoginLogItem{
			Id: row.Id, UserId: row.UserId.Int64, Username: row.Username.String,
			Ip: row.Ip.String, Ua: row.Ua.String, Success: row.Success.Int64,
			Msg: row.Msg.String, LoginAt: row.LoginAt,
		})
		nextCursor = row.Id
	}
	return &system.LoginLogListResp{Base: responsePage(total, hasNext, nextCursor, cursor > 0, 0), Data: data}, nil
}
