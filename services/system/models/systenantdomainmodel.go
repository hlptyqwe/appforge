package models

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	TenantDomainStatusActive   int64 = 1
	TenantDomainStatusRetired  int64 = 2
	TenantDomainStatusDisabled int64 = 3
)

type SysTenantDomainModel interface {
	FindByTenantOrigin(ctx context.Context, tenantId int64, origin string) (*SysTenantDomain, error)
	FindHighestPriorityActive(ctx context.Context, tenantId int64) (*SysTenantDomain, error)
	FindOne(ctx context.Context, id int64) (*SysTenantDomain, error)
	FindAllByTenant(ctx context.Context, tenantId int64) ([]*SysTenantDomain, error)
	CountActive(ctx context.Context, tenantId int64, excludeId int64) (int64, error)
	Insert(ctx context.Context, data *SysTenantDomain) error
	Update(ctx context.Context, data *SysTenantDomain) error
	Delete(ctx context.Context, id int64) error
}

type customSysTenantDomainModel struct {
	conn sqlx.SqlConn
}

func NewSysTenantDomainModel(conn sqlx.SqlConn) SysTenantDomainModel {
	return &customSysTenantDomainModel{conn: conn}
}

func (m *customSysTenantDomainModel) FindByTenantOrigin(ctx context.Context, tenantId int64, origin string) (*SysTenantDomain, error) {
	var result SysTenantDomain
	err := m.conn.QueryRowCtx(ctx, &result, `SELECT id, tenant_id, origin, status, priority, create_times, update_times
		FROM sys_tenant_domain WHERE tenant_id = ? AND origin = ? LIMIT 1`, tenantId, origin)
	return &result, err
}

func (m *customSysTenantDomainModel) FindHighestPriorityActive(ctx context.Context, tenantId int64) (*SysTenantDomain, error) {
	var result SysTenantDomain
	err := m.conn.QueryRowCtx(ctx, &result, `SELECT id, tenant_id, origin, status, priority, create_times, update_times
		FROM sys_tenant_domain WHERE tenant_id = ? AND status = ? ORDER BY priority DESC, id ASC LIMIT 1`,
		tenantId, TenantDomainStatusActive)
	return &result, err
}

func (m *customSysTenantDomainModel) FindOne(ctx context.Context, id int64) (*SysTenantDomain, error) {
	var result SysTenantDomain
	err := m.conn.QueryRowCtx(ctx, &result, `SELECT id, tenant_id, origin, status, priority, create_times, update_times
		FROM sys_tenant_domain WHERE id = ? LIMIT 1`, id)
	return &result, err
}

func (m *customSysTenantDomainModel) FindAllByTenant(ctx context.Context, tenantId int64) ([]*SysTenantDomain, error) {
	var result []*SysTenantDomain
	err := m.conn.QueryRowsCtx(ctx, &result, `SELECT id, tenant_id, origin, status, priority, create_times, update_times
		FROM sys_tenant_domain WHERE tenant_id = ? ORDER BY status ASC, priority DESC, id DESC`, tenantId)
	return result, err
}

func (m *customSysTenantDomainModel) CountActive(ctx context.Context, tenantId int64, excludeId int64) (int64, error) {
	var count int64
	err := m.conn.QueryRowCtx(ctx, &count, `SELECT COUNT(1) FROM sys_tenant_domain
		WHERE tenant_id = ? AND status = ? AND id <> ?`, tenantId, TenantDomainStatusActive, excludeId)
	return count, err
}

func (m *customSysTenantDomainModel) Insert(ctx context.Context, data *SysTenantDomain) error {
	_, err := m.conn.ExecCtx(ctx, `INSERT INTO sys_tenant_domain
		(tenant_id, origin, status, priority, create_times, update_times) VALUES (?, ?, ?, ?, ?, ?)`,
		data.TenantId, data.Origin, data.Status, data.Priority, data.CreateTimes, data.UpdateTimes)
	return err
}

func (m *customSysTenantDomainModel) Update(ctx context.Context, data *SysTenantDomain) error {
	_, err := m.conn.ExecCtx(ctx, `UPDATE sys_tenant_domain SET origin = ?, status = ?, priority = ?, update_times = ? WHERE id = ?`,
		data.Origin, data.Status, data.Priority, data.UpdateTimes, data.Id)
	return err
}

func (m *customSysTenantDomainModel) Delete(ctx context.Context, id int64) error {
	_, err := m.conn.ExecCtx(ctx, `DELETE FROM sys_tenant_domain WHERE id = ?`, id)
	return err
}
