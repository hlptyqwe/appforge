-- AppForge V4 构建集群管理菜单、取消重试和接口权限。
SET NAMES utf8mb4;
SET time_zone = '+08:00';

INSERT INTO sys_menu (
  id, parent_id, app_scope, name, menu_type, method, path, component, perms,
  icon, sort, visible, enabled, create_times, update_times
) VALUES
  (16, 1, 1, '构建集群', 2, '', '/platform/build-cluster', 'platform/build-cluster', '', 'Monitor', 70, 1, 1, 0, 0),
  (1044, 14, 1, '取消构建任务', 3, 'POST', '/core/build-tasks/:id/cancel', '', 'core:build-task:cancel', '', 4, 2, 1, 0, 0),
  (1045, 14, 1, '重试构建任务', 3, 'POST', '/core/build-tasks/:id/retry', '', 'core:build-task:retry', '', 5, 2, 1, 0, 0),
  (1071, 16, 1, '查看Builder节点', 3, 'GET', '/core/build-cluster/nodes', '', 'core:build-cluster:view', '', 1, 2, 1, 0, 0),
  (1072, 16, 1, '排空Builder节点', 3, 'POST', '/core/build-cluster/nodes/:id/drain', '', 'core:build-cluster:drain', '', 2, 2, 1, 0, 0),
  (1073, 16, 1, '查看并发策略', 3, 'GET', '/core/build-cluster/policies', '', 'core:build-cluster:view', '', 3, 2, 1, 0, 0),
  (1074, 16, 1, '保存并发策略', 3, 'POST', '/core/build-cluster/policies', '', 'core:build-cluster:policy', '', 4, 2, 1, 0, 0),
  (1075, 16, 1, '查看构建缓存', 3, 'GET', '/core/build-cluster/cache', '', 'core:build-cluster:view', '', 5, 2, 1, 0, 0),
  (1076, 16, 1, '失效构建缓存', 3, 'POST', '/core/build-cluster/cache/:id/invalidate', '', 'core:build-cluster:cache', '', 6, 2, 1, 0, 0),
  (1077, 16, 1, '查看调度事件', 3, 'GET', '/core/build-cluster/events', '', 'core:build-cluster:view', '', 7, 2, 1, 0, 0),
  (1078, 16, 1, '清理构建缓存', 3, 'POST', '/core/build-cluster/cache/cleanup', '', 'core:build-cluster:cache', '', 8, 2, 1, 0, 0),
  (1079, 16, 1, '查看集群指标', 3, 'GET', '/core/build-cluster/metrics', '', 'core:build-cluster:view', '', 9, 2, 1, 0, 0),
  (3245, 3050, 2, '取消构建任务', 3, 'POST', '/core/build-tasks/:id/cancel', '', 'core:build-task:cancel', '', 5, 2, 1, 0, 0),
  (3246, 3050, 2, '重试构建任务', 3, 'POST', '/core/build-tasks/:id/retry', '', 'core:build-task:retry', '', 6, 2, 1, 0, 0)
ON DUPLICATE KEY UPDATE name = VALUES(name), parent_id = VALUES(parent_id), app_scope = VALUES(app_scope),
  method = VALUES(method), path = VALUES(path), component = VALUES(component), perms = VALUES(perms),
  icon = VALUES(icon), sort = VALUES(sort), visible = VALUES(visible), enabled = VALUES(enabled);

INSERT IGNORE INTO sys_role_menu (tenant_id, role_id, menu_id)
SELECT r.tenant_id, r.id, m.id
FROM sys_role r
JOIN sys_menu m ON m.app_scope = r.app_scope
WHERE r.app_scope = 1
  AND r.code IN ('owner', 'admin')
  AND m.id IN (16, 1044, 1045, 1071, 1072, 1073, 1074, 1075, 1076, 1077, 1078, 1079);

INSERT IGNORE INTO sys_role_menu (tenant_id, role_id, menu_id)
SELECT r.tenant_id, r.id, m.id
FROM sys_role r
JOIN sys_menu m ON m.app_scope = r.app_scope
WHERE r.app_scope = 1
  AND r.code = 'viewer'
  AND m.id IN (16, 1071, 1073, 1075, 1077, 1079);

INSERT IGNORE INTO sys_role_menu (tenant_id, role_id, menu_id)
SELECT r.tenant_id, r.id, m.id
FROM sys_role r
JOIN sys_menu m ON m.app_scope = r.app_scope
WHERE r.app_scope = 2
  AND r.code IN ('owner', 'admin')
  AND m.id IN (3245, 3246);

INSERT INTO sys_schema_migration (version, description)
VALUES ('20260814_91_v4_build_cluster_menu', 'V4构建集群管理菜单、取消重试、指标与接口权限')
ON DUPLICATE KEY UPDATE description = VALUES(description);
