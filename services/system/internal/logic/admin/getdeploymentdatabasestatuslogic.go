package adminlogic

import (
	"context"
	"time"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetDeploymentDatabaseStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetDeploymentDatabaseStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDeploymentDatabaseStatusLogic {
	return &GetDeploymentDatabaseStatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 查询数据库实际迁移状态，供企业部署只读诊断页面使用。
func (l *GetDeploymentDatabaseStatusLogic) GetDeploymentDatabaseStatus(in *system.Empty) (*system.DeploymentDatabaseStatusResp, error) {
	var migrationCount int64
	if err := l.svcCtx.DB.QueryRowCtx(l.ctx, &migrationCount, "SELECT COUNT(*) FROM sys_schema_migration"); err != nil {
		return nil, err
	}
	type migrationRow struct {
		Version     string `db:"version"`
		Description string `db:"description"`
	}
	var rows []migrationRow
	if err := l.svcCtx.DB.QueryRowsCtx(l.ctx, &rows,
		"SELECT version,description FROM sys_schema_migration ORDER BY version DESC LIMIT 10"); err != nil {
		return nil, err
	}
	migrations := make([]*system.DeploymentMigrationItem, 0, len(rows))
	for _, row := range rows {
		migrations = append(migrations, &system.DeploymentMigrationItem{
			Version: row.Version, Description: row.Description,
		})
	}
	latest := ""
	if len(rows) > 0 {
		latest = rows[0].Version
	}

	return &system.DeploymentDatabaseStatusResp{
		Base: responseBase(), LatestSchemaVersion: latest, MigrationCount: migrationCount,
		RecentMigrations: migrations, CheckedAt: time.Now().UnixMilli(),
	}, nil
}
