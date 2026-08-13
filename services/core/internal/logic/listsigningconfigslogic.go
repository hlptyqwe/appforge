package logic

import (
	"context"
	"fmt"
	"strings"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"
	"appforge/services/core/models"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ListSigningConfigsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListSigningConfigsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSigningConfigsLogic {
	return &ListSigningConfigsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListSigningConfigsLogic) ListSigningConfigs(in *core.SigningConfigListReq) (*core.SigningConfigListResp, error) {
	tenant, err := tenantID(l.ctx)
	if err != nil {
		return nil, err
	}
	cursor, limit := pageValues(in.GetPage())
	where := []string{"tenant_id = ?"}
	args := []any{tenant}
	if in.GetAppId() > 0 {
		where = append(where, "app_id = ?")
		args = append(args, in.GetAppId())
	}
	if in.GetStatus() > 0 {
		where = append(where, "status = ?")
		args = append(args, in.GetStatus())
	}
	whereSQL := strings.Join(where, " AND ")
	var total int64
	if err := l.svcCtx.DB.QueryRowCtx(l.ctx, &total, fmt.Sprintf("SELECT COUNT(1) FROM t_app_signing_config WHERE %s", whereSQL), args...); err != nil {
		return nil, status.Errorf(codes.Internal, "list signing configs count failed: %v", err)
	}
	queryArgs := append(append([]any{}, args...), cursor, limit+1)
	var items []models.TAppSigningConfig
	query := fmt.Sprintf("SELECT id, tenant_id, app_id, name, keystore_object_key, key_alias, keystore_password_ciphertext, key_password_ciphertext, secret_ref, status, last_verified_at, create_by, create_time, update_time FROM t_app_signing_config WHERE %s AND id > ? ORDER BY id ASC LIMIT ?", whereSQL)
	if err := l.svcCtx.DB.QueryRowsCtx(l.ctx, &items, query, queryArgs...); err != nil {
		return nil, status.Errorf(codes.Internal, "list signing configs failed: %v", err)
	}
	hasNext := int64(len(items)) > limit
	if hasNext {
		items = items[:limit]
	}
	data := make([]*core.SigningConfig, 0, len(items))
	var nextCursor int64
	for i := range items {
		data = append(data, mapSigningConfig(&items[i]))
		nextCursor = items[i].Id
	}
	if !hasNext {
		nextCursor = 0
	}

	return &core.SigningConfigListResp{Base: baseWithTotal(total, hasNext, nextCursor), Data: data}, nil
}
