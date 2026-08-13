package models

import (
	"appforge/common/sqlutil"
	"context"
	"database/sql"
	"fmt"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"time"
)

var _ SysUserModel = (*customSysUserModel)(nil)

type (
	UserPageFilter struct {
		Keyword  string
		TenantId int64
		Enabled  int64
		AppScope int64
	}

	// SysUserModel is an interface to be customized, add more methods here,
	// and implement the added methods in customSysUserModel.
	SysUserModel interface {
		sysUserModel
		FindOneByUsername(ctx context.Context, username string, appScope int64) (*SysUser, error)
		FindIdsByTenantId(ctx context.Context, tenantId int64) ([]int64, error)
		FindPage(ctx context.Context, filter UserPageFilter, cursor int64, limit int64) ([]*SysUser, int64, error)
		TransCtx(ctx context.Context, fn func(context context.Context, session sqlx.Session) error) error
		InsertCtx(ctx context.Context, session sqlx.Session, data *SysUser) (sql.Result, error)
		InsertGoogle2FASecret(ctx context.Context, userId int64, secret string) error
		GetGoogle2FASecret(ctx context.Context, userId int64) (string, error)
		DeleteGoogle2FASecret(ctx context.Context, userId int64) error
	}

	customSysUserModel struct {
		*defaultSysUserModel
	}
)

// NewSysUserModel returns a model for the database table.
func NewSysUserModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) SysUserModel {
	return &customSysUserModel{
		defaultSysUserModel: newSysUserModel(conn, c, opts...),
	}
}

var (
	google2FASecretCachePrefix = "cache:google2FASecret:userId:"
)

func (m *defaultSysUserModel) FindOneByUsername(ctx context.Context, username string, appScope int64) (*SysUser, error) {
	var resp SysUser
	query := fmt.Sprintf("select %s from %s where `username` = ? and `app_scope` = ? limit 1", sysUserRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &resp, query, username, appScope)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *defaultSysUserModel) FindIdsByTenantId(ctx context.Context, tenantId int64) ([]int64, error) {
	builder := sqlutil.NewPageQueryBuilder()
	builder.EqInt64("tenant_id", tenantId)

	var ids []int64
	query := fmt.Sprintf("SELECT id FROM %s WHERE %s", m.table, builder.Where())
	err := m.QueryRowsNoCacheCtx(ctx, &ids, query, builder.Args()...)
	return ids, err
}

func (m *defaultSysUserModel) FindPage(
	ctx context.Context,
	filter UserPageFilter,
	cursor int64,
	limit int64,
) ([]*SysUser, int64, error) {

	limit = sqlutil.NormalizeLimit(limit)

	// ---- WHERE 条件 ----
	builder := sqlutil.NewPageQueryBuilder()
	if filter.Keyword != "" {
		like := "%" + filter.Keyword + "%"
		builder.And("(username LIKE ? OR nickname LIKE ?)", like, like)
	}
	builder.EqInt64("enabled", filter.Enabled)
	builder.EqInt64("tenant_id", filter.TenantId)
	builder.EqInt64("app_scope", filter.AppScope)

	where := builder.Where()
	args := builder.Args()

	// ---- total ----
	var total int64
	countSql := fmt.Sprintf("SELECT COUNT(1) FROM %s WHERE %s", m.table, where)
	if err := m.QueryRowNoCacheCtx(ctx, &total, countSql, args...); err != nil {
		return nil, 0, err
	}

	// ---- list ----
	var listSql string
	listArgs := append([]any{}, args...)

	if cursor <= 0 {
		// 第一页
		listSql = fmt.Sprintf(
			"SELECT %s FROM %s WHERE %s ORDER BY id DESC LIMIT ?",
			sysUserRows, m.table, where,
		)
		listArgs = append(listArgs, limit)
	} else {
		// 后续页
		listSql = fmt.Sprintf(
			"SELECT %s FROM %s WHERE %s AND id < ? ORDER BY id DESC LIMIT ?",
			sysUserRows, m.table, where,
		)
		listArgs = append(listArgs, cursor, limit)
	}

	var list []*SysUser
	if err := m.QueryRowsNoCacheCtx(ctx, &list, listSql, listArgs...); err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

func (m *defaultSysUserModel) TransCtx(ctx context.Context, fn func(context context.Context, session sqlx.Session) error) error {
	return m.CachedConn.TransactCtx(ctx, fn)
}

func (m *defaultSysUserModel) InsertCtx(ctx context.Context, session sqlx.Session, data *SysUser) (sql.Result, error) {
	query := fmt.Sprintf(
		"insert into %s (`tenant_id`, `app_scope`, `user_type`, `is_owner`, `username`, `password`, `nickname`, `avatar`, `enabled`, `google_secret`, `google_enabled`, `perms_ver`, `last_login_ip`, `last_login_at`, `create_by`, `create_times`, `update_times`) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		m.table,
	)
	ret, err := session.ExecCtx(
		ctx,
		query,
		data.TenantId,
		data.AppScope,
		data.UserType,
		data.IsOwner,
		data.Username,
		data.Password,
		data.Nickname,
		data.Avatar,
		data.Enabled,
		data.GoogleSecret,
		data.GoogleEnabled,
		data.PermsVer,
		data.LastLoginIp,
		data.LastLoginAt,
		data.CreateBy,
		data.CreateTimes,
		data.UpdateTimes,
	)
	return ret, err
}

func (m *defaultSysUserModel) InsertGoogle2FASecret(ctx context.Context, userId int64, secret string) error {
	key := fmt.Sprintf("%s%v", google2FASecretCachePrefix, userId)
	return m.SetCacheWithExpireCtx(ctx, key, secret, 10*time.Minute)
}

func (m *defaultSysUserModel) GetGoogle2FASecret(ctx context.Context, userId int64) (string, error) {
	key := fmt.Sprintf("%s%v", google2FASecretCachePrefix, userId)
	var secret string
	err := m.GetCacheCtx(ctx, key, &secret)
	return secret, err
}

func (m *defaultSysUserModel) DeleteGoogle2FASecret(ctx context.Context, userId int64) error {
	key := fmt.Sprintf("%s%v", google2FASecretCachePrefix, userId)
	return m.DelCache(key)
}
