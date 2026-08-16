-- AppForge V1 代理端作用域、菜单、权限与本地验收账号。
SET NAMES utf8mb4;
SET time_zone = '+08:00';

INSERT INTO sys_menu (
  id, parent_id, app_scope, name, menu_type, method, path, component, perms,
  icon, sort, visible, enabled, create_times, update_times
) VALUES
  (3000, 0, 2, '业务中心', 1, '', '', '', '', 'Box', 10, 1, 1, 0, 0),
  (3010, 3000, 2, '应用管理', 2, '', '/platform/applications', 'platform/applications', '', 'Grid', 10, 1, 1, 0, 0),
  (3020, 3000, 2, '版本管理', 2, '', '/platform/versions', 'platform/versions', '', 'Collection', 20, 1, 1, 0, 0),
  (3030, 3000, 2, '签名管理', 2, '', '/platform/signing-configs', 'platform/signing-configs', '', 'Key', 30, 1, 1, 0, 0),
  (3040, 3000, 2, '渠道管理', 2, '', '/platform/channels', 'platform/channels', '', 'Promotion', 40, 1, 1, 0, 0),
  (3050, 3000, 2, '构建中心', 2, '', '/platform/build-tasks', 'platform/build-tasks', '', 'SetUp', 50, 1, 1, 0, 0),
  (3060, 3000, 2, '渠道统计', 2, '', '/platform/channel-stats', 'platform/channel-stats', '', 'DataAnalysis', 60, 1, 1, 0, 0),
  (3100, 0, 2, '团队', 1, '', '', '', '', 'UserFilled', 20, 1, 1, 0, 0),
  (3110, 3100, 2, '成员', 2, '', '/team/users', 'system/users', '', 'User', 10, 1, 1, 0, 0),
  (3120, 3100, 2, '角色', 2, '', '/team/roles', 'system/roles', '', 'UserFilled', 20, 1, 1, 0, 0),

  (3201, 3010, 2, '查看应用列表', 3, 'GET', '/core/applications', '', 'core:application:view', '', 1, 2, 1, 0, 0),
  (3202, 3010, 2, '查看应用详情', 3, 'GET', '/core/applications/:id', '', 'core:application:view', '', 2, 2, 1, 0, 0),
  (3203, 3010, 2, '创建应用', 3, 'POST', '/core/applications', '', 'core:application:add', '', 3, 2, 1, 0, 0),
  (3211, 3020, 2, '查看版本列表', 3, 'GET', '/core/versions', '', 'core:version:view', '', 1, 2, 1, 0, 0),
  (3212, 3020, 2, '查看版本详情', 3, 'GET', '/core/versions/:id', '', 'core:version:view', '', 2, 2, 1, 0, 0),
  (3213, 3020, 2, '创建版本', 3, 'POST', '/core/versions', '', 'core:version:add', '', 3, 2, 1, 0, 0),
  (3214, 3020, 2, '初始化文件上传', 3, 'POST', '/core/uploads/initiate', '', 'core:storage:upload', '', 4, 2, 1, 0, 0),
  (3215, 3020, 2, '完成文件上传', 3, 'POST', '/core/uploads/:id/complete', '', 'core:storage:upload', '', 5, 2, 1, 0, 0),
  (3221, 3030, 2, '查看签名配置', 3, 'GET', '/core/signing-configs', '', 'core:signing-config:view', '', 1, 2, 1, 0, 0),
  (3222, 3030, 2, '查看签名详情', 3, 'GET', '/core/signing-configs/:id', '', 'core:signing-config:view', '', 2, 2, 1, 0, 0),
  (3223, 3030, 2, '创建签名配置', 3, 'POST', '/core/signing-configs', '', 'core:signing-config:add', '', 3, 2, 1, 0, 0),
  (3231, 3040, 2, '查看渠道列表', 3, 'GET', '/core/channels', '', 'core:channel:view', '', 1, 2, 1, 0, 0),
  (3232, 3040, 2, '查看渠道详情', 3, 'GET', '/core/channels/:id', '', 'core:channel:view', '', 2, 2, 1, 0, 0),
  (3233, 3040, 2, '创建渠道', 3, 'POST', '/core/channels', '', 'core:channel:add', '', 3, 2, 1, 0, 0),
  (3241, 3050, 2, '查看构建任务', 3, 'GET', '/core/build-tasks', '', 'core:build-task:view', '', 1, 2, 1, 0, 0),
  (3242, 3050, 2, '查看构建详情', 3, 'GET', '/core/build-tasks/:id', '', 'core:build-task:view', '', 2, 2, 1, 0, 0),
  (3243, 3050, 2, '创建构建任务', 3, 'POST', '/core/build-tasks', '', 'core:build-task:add', '', 3, 2, 1, 0, 0),
  (3244, 3050, 2, '下载构建产物', 3, 'GET', '/core/storage/objects/:id/download', '', 'core:storage:download', '', 4, 2, 1, 0, 0),
  (3251, 3060, 2, '查看渠道统计', 3, 'GET', '/core/channel-stats', '', 'core:channel-stats:view', '', 1, 2, 1, 0, 0),

  (3301, 3110, 2, '查看成员', 3, 'GET', '/team/users', '', 'sys:user:view', '', 1, 2, 1, 0, 0),
  (3308, 3110, 2, '查看成员详情', 3, 'GET', '/team/users/:id', '', 'sys:user:view', '', 8, 2, 1, 0, 0),
  (3302, 3110, 2, '创建成员', 3, 'POST', '/team/users', '', 'sys:user:add', '', 2, 2, 1, 0, 0),
  (3303, 3110, 2, '修改成员', 3, 'PUT', '/team/users', '', 'sys:user:update', '', 3, 2, 1, 0, 0),
  (3304, 3110, 2, '删除成员', 3, 'DELETE', '/team/users/:id', '', 'sys:user:delete', '', 4, 2, 1, 0, 0),
  (3305, 3110, 2, '修改成员状态', 3, 'POST', '/team/users/status', '', 'sys:user:status', '', 5, 2, 1, 0, 0),
  (3306, 3110, 2, '重置成员密码', 3, 'POST', '/team/users/resetPwd', '', 'sys:user:resetpwd', '', 6, 2, 1, 0, 0),
  (3307, 3110, 2, '分配成员角色', 3, 'POST', '/team/users/assignRoles', '', 'sys:user:assignrole', '', 7, 2, 1, 0, 0),
  (3311, 3120, 2, '查看角色', 3, 'GET', '/team/roles', '', 'sys:role:view', '', 1, 2, 1, 0, 0),
  (3312, 3120, 2, '创建角色', 3, 'POST', '/team/roles', '', 'sys:role:add', '', 2, 2, 1, 0, 0),
  (3313, 3120, 2, '修改角色', 3, 'PUT', '/team/roles', '', 'sys:role:update', '', 3, 2, 1, 0, 0),
  (3314, 3120, 2, '删除角色', 3, 'DELETE', '/team/roles/:id', '', 'sys:role:delete', '', 4, 2, 1, 0, 0),
  (3315, 3120, 2, '查看角色授权', 3, 'GET', '/team/roles/:id/grant', '', 'sys:role:grant:detail', '', 5, 2, 1, 0, 0),
  (3316, 3120, 2, '角色授权', 3, 'POST', '/team/roles/grant', '', 'sys:role:grant', '', 6, 2, 1, 0, 0),
  (3317, 3120, 2, '查看权限', 3, 'GET', '/team/perms', '', 'sys:role:view', '', 7, 2, 1, 0, 0),
  (3318, 3120, 2, '查看菜单树', 3, 'GET', '/team/menus/tree/:roleId', '', 'sys:role:grant:detail', '', 8, 2, 1, 0, 0)
ON DUPLICATE KEY UPDATE name = VALUES(name), parent_id = VALUES(parent_id), app_scope = VALUES(app_scope),
  method = VALUES(method), path = VALUES(path), component = VALUES(component), perms = VALUES(perms),
  icon = VALUES(icon), sort = VALUES(sort), visible = VALUES(visible), enabled = VALUES(enabled);

INSERT INTO sys_role (id, tenant_id, app_scope, name, code, enabled, remark, create_times, update_times)
VALUES (1001, 1, 2, '所有者', 'owner', 1, '代理端租户所有者',
  UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3)) * 1000, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3)) * 1000)
ON DUPLICATE KEY UPDATE name = VALUES(name), enabled = VALUES(enabled), remark = VALUES(remark);

-- 本地代理端验收账号：agent / AppForge@123。
INSERT INTO sys_user (
  id, tenant_id, app_scope, user_type, is_owner, username, password, nickname,
  avatar, enabled, google_secret, google_enabled, perms_ver, last_login_ip,
  last_login_at, create_by, create_times, update_times
) VALUES (
  1001, 1, 2, 2, 1, 'agent',
  '$2y$10$bxPB8yeV4QLuCn5mNxzJB.cMVDtGXpRZiIF.r/u6c09RDBeUDGgaC',
  '代理端所有者', '', 1, '', 2, 1, '', 0, 0,
  UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3)) * 1000,
  UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3)) * 1000
) ON DUPLICATE KEY UPDATE enabled = 1, app_scope = 2, user_type = 2, is_owner = 1;

INSERT IGNORE INTO sys_user_role (tenant_id, user_id, role_id) VALUES (1, 1001, 1001);
INSERT IGNORE INTO sys_role_menu (tenant_id, role_id, menu_id)
-- 仅授予本迁移定义的V1代理端权限，禁止迁移重放时自动吸收后续敏感权限。
SELECT 1, 1001, id FROM sys_menu WHERE app_scope = 2 AND id BETWEEN 3000 AND 3318;

INSERT INTO sys_schema_migration (version, description)
VALUES ('20260813_50_agent_portal', 'V1代理端身份域、菜单权限和本地验收账号')
ON DUPLICATE KEY UPDATE description = VALUES(description);
