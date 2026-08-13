-- ============================================================================
-- AppForge 开发环境基础数据
-- 仅初始化租户、管理员、RBAC 菜单权限和系统配置，不写入业务演示数据。
-- ============================================================================

SET NAMES utf8mb4;
SET time_zone = '+08:00';

-- 默认开发租户。
INSERT INTO sys_tenant (
  id, tenant_code, tenant_name, enabled, expire_time, contact_name, contact_phone,
  remark, create_by, create_times, update_by, update_times
) VALUES (
  1, 'default', 'AppForge 开发租户', 1, 0, '开发管理员', '',
  '开发环境初始化租户', 'system', UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3)) * 1000,
  'system', UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3)) * 1000
);

INSERT INTO sys_tenant_domain (
  id, tenant_id, origin, status, priority, create_times, update_times
) VALUES
  (1, 1, 'http://localhost:5173', 1, 100,
   UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3)) * 1000,
   UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3)) * 1000);

-- 轻量 RBAC：owner 拥有全部权限，admin 管理业务，viewer 只读业务数据。
INSERT INTO sys_role (
  id, tenant_id, app_scope, name, code, enabled, remark, create_times, update_times
) VALUES
  (1, 1, 1, '所有者', 'owner', 1, '租户所有者，拥有全部菜单和接口权限',
   UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3)) * 1000, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3)) * 1000),
  (2, 1, 1, '业务管理员', 'admin', 1, '管理应用、版本、渠道、签名配置和构建任务',
   UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3)) * 1000, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3)) * 1000),
  (3, 1, 1, '只读用户', 'viewer', 1, '只允许查看业务数据和统计记录',
   UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3)) * 1000, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3)) * 1000);

-- 默认账号：appforge / AppForge@123。密码使用 bcrypt 存储。
INSERT INTO sys_user (
  id, tenant_id, app_scope, user_type, is_owner, username, password, nickname,
  avatar, enabled, google_secret, google_enabled, perms_ver, last_login_ip,
  last_login_at, create_by, create_times, update_times
) VALUES (
  1, 1, 1, 2, 1, 'appforge',
  '$2y$10$bxPB8yeV4QLuCn5mNxzJB.cMVDtGXpRZiIF.r/u6c09RDBeUDGgaC',
  'AppForge Owner', '', 1, '', 2, 1, '', 0, 0,
  UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3)) * 1000,
  UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3)) * 1000
);

INSERT INTO sys_user_role (tenant_id, user_id, role_id) VALUES (1, 1, 1);

-- 前端业务菜单。
INSERT INTO sys_menu (
  id, parent_id, app_scope, name, menu_type, method, path, component, perms,
  icon, sort, visible, enabled, create_times, update_times
) VALUES
  (1, 0, 1, '渠道打包', 1, '', '', '', '', 'Box', 10, 1, 1, 0, 0),
  (10, 1, 1, '应用管理', 2, '', '/platform/applications', 'platform/applications', '', 'Grid', 10, 1, 1, 0, 0),
  (11, 1, 1, '版本管理', 2, '', '/platform/versions', 'platform/versions', '', 'Collection', 20, 1, 1, 0, 0),
  (12, 1, 1, '渠道管理', 2, '', '/platform/channels', 'platform/channels', '', 'Promotion', 30, 1, 1, 0, 0),
  (13, 1, 1, '签名配置', 2, '', '/platform/signing-configs', 'platform/signing-configs', '', 'Key', 40, 1, 1, 0, 0),
  (14, 1, 1, '构建任务', 2, '', '/platform/build-tasks', 'platform/build-tasks', '', 'SetUp', 50, 1, 1, 0, 0),
  (15, 1, 1, '渠道统计', 2, '', '/platform/channel-stats', 'platform/channel-stats', '', 'DataAnalysis', 60, 1, 1, 0, 0),

  (2, 0, 1, '系统管理', 1, '', '', '', '', 'Setting', 20, 1, 1, 0, 0),
  (20, 2, 1, '用户管理', 2, '', '/system/users', 'system/users', '', 'User', 10, 1, 1, 0, 0),
  (21, 2, 1, '角色管理', 2, '', '/system/roles', 'system/roles', '', 'UserFilled', 20, 1, 1, 0, 0),
  (22, 2, 1, '菜单权限', 2, '', '/system/menus', 'system/menus', '', 'Menu', 30, 1, 1, 0, 0),
  (23, 2, 1, '租户管理', 2, '', '/system/tenants', 'system/tenants', '', 'OfficeBuilding', 40, 1, 1, 0, 0),
  (24, 2, 1, '租户域名', 2, '', '/system/tenant-domains', 'system/tenant-domains', '', 'Link', 50, 1, 1, 0, 0),
  (25, 2, 1, '系统配置', 2, '', '/system/configs', 'system/configs', '', 'Tools', 60, 1, 1, 0, 0),
  (26, 2, 1, '登录日志', 2, '', '/system/login-log', 'system/login-log', '', 'Document', 70, 1, 1, 0, 0),
  (27, 2, 1, '操作日志', 2, '', '/system/op-log', 'system/op-log', '', 'Tickets', 80, 1, 1, 0, 0);

-- Core 接口权限。path 使用 API RBAC 中去掉 /admin 后的路径。
INSERT INTO sys_menu (
  id, parent_id, app_scope, name, menu_type, method, path, component, perms,
  icon, sort, visible, enabled, create_times, update_times
) VALUES
  (1001, 10, 1, '查看应用列表', 3, 'GET', '/core/applications', '', 'core:application:view', '', 1, 2, 1, 0, 0),
  (1002, 10, 1, '查看应用详情', 3, 'GET', '/core/applications/:id', '', 'core:application:view', '', 2, 2, 1, 0, 0),
  (1003, 10, 1, '创建应用', 3, 'POST', '/core/applications', '', 'core:application:add', '', 3, 2, 1, 0, 0),
  (1011, 11, 1, '查看版本列表', 3, 'GET', '/core/versions', '', 'core:version:view', '', 1, 2, 1, 0, 0),
  (1012, 11, 1, '查看版本详情', 3, 'GET', '/core/versions/:id', '', 'core:version:view', '', 2, 2, 1, 0, 0),
  (1013, 11, 1, '创建版本', 3, 'POST', '/core/versions', '', 'core:version:add', '', 3, 2, 1, 0, 0),
  (1021, 12, 1, '查看渠道列表', 3, 'GET', '/core/channels', '', 'core:channel:view', '', 1, 2, 1, 0, 0),
  (1022, 12, 1, '查看渠道详情', 3, 'GET', '/core/channels/:id', '', 'core:channel:view', '', 2, 2, 1, 0, 0),
  (1023, 12, 1, '创建渠道', 3, 'POST', '/core/channels', '', 'core:channel:add', '', 3, 2, 1, 0, 0),
  (1031, 13, 1, '查看签名配置列表', 3, 'GET', '/core/signing-configs', '', 'core:signing-config:view', '', 1, 2, 1, 0, 0),
  (1032, 13, 1, '查看签名配置详情', 3, 'GET', '/core/signing-configs/:id', '', 'core:signing-config:view', '', 2, 2, 1, 0, 0),
  (1033, 13, 1, '创建签名配置', 3, 'POST', '/core/signing-configs', '', 'core:signing-config:add', '', 3, 2, 1, 0, 0),
  (1041, 14, 1, '查看构建任务列表', 3, 'GET', '/core/build-tasks', '', 'core:build-task:view', '', 1, 2, 1, 0, 0),
  (1042, 14, 1, '查看构建任务详情', 3, 'GET', '/core/build-tasks/:id', '', 'core:build-task:view', '', 2, 2, 1, 0, 0),
  (1043, 14, 1, '创建构建任务', 3, 'POST', '/core/build-tasks', '', 'core:build-task:add', '', 3, 2, 1, 0, 0),
  (1051, 15, 1, '查看渠道统计', 3, 'GET', '/core/channel-stats', '', 'core:channel-stats:view', '', 1, 2, 1, 0, 0);

-- System 接口权限。
INSERT INTO sys_menu (
  id, parent_id, app_scope, name, menu_type, method, path, component, perms,
  icon, sort, visible, enabled, create_times, update_times
) VALUES
  (2001, 20, 1, '查看用户列表', 3, 'GET', '/system/users', '', 'system:user:view', '', 1, 2, 1, 0, 0),
  (2002, 20, 1, '查看用户详情', 3, 'GET', '/system/users/detail', '', 'system:user:view', '', 2, 2, 1, 0, 0),
  (2003, 20, 1, '创建用户', 3, 'POST', '/system/users', '', 'system:user:add', '', 3, 2, 1, 0, 0),
  (2004, 20, 1, '修改用户', 3, 'PUT', '/system/users', '', 'system:user:update', '', 4, 2, 1, 0, 0),
  (2005, 20, 1, '删除用户', 3, 'DELETE', '/system/users/:id', '', 'system:user:delete', '', 5, 2, 1, 0, 0),
  (2006, 20, 1, '修改用户状态', 3, 'POST', '/system/users/status', '', 'system:user:update', '', 6, 2, 1, 0, 0),
  (2007, 20, 1, '重置用户密码', 3, 'POST', '/system/users/resetPwd', '', 'system:user:reset-password', '', 7, 2, 1, 0, 0),
  (2008, 20, 1, '分配用户角色', 3, 'POST', '/system/users/assignRoles', '', 'system:user:assign-role', '', 8, 2, 1, 0, 0),
  (2009, 20, 1, '初始化用户二次验证', 3, 'POST', '/system/users/google2fa/init', '', 'system:user:google2fa', '', 9, 2, 1, 0, 0),
  (2010, 20, 1, '绑定用户二次验证', 3, 'POST', '/system/users/google2fa/bind', '', 'system:user:google2fa', '', 10, 2, 1, 0, 0),
  (2011, 20, 1, '启用用户二次验证', 3, 'POST', '/system/users/google2fa/enable', '', 'system:user:google2fa', '', 11, 2, 1, 0, 0),
  (2012, 20, 1, '停用用户二次验证', 3, 'POST', '/system/users/google2fa/disable', '', 'system:user:google2fa', '', 12, 2, 1, 0, 0),
  (2013, 20, 1, '重置用户二次验证', 3, 'POST', '/system/users/google2fa/reset', '', 'system:user:google2fa', '', 13, 2, 1, 0, 0),

  (2021, 21, 1, '查看角色列表', 3, 'GET', '/system/roles', '', 'system:role:view', '', 1, 2, 1, 0, 0),
  (2022, 21, 1, '创建角色', 3, 'POST', '/system/roles', '', 'system:role:add', '', 2, 2, 1, 0, 0),
  (2023, 21, 1, '修改角色', 3, 'PUT', '/system/roles', '', 'system:role:update', '', 3, 2, 1, 0, 0),
  (2024, 21, 1, '删除角色', 3, 'DELETE', '/system/roles/:id', '', 'system:role:delete', '', 4, 2, 1, 0, 0),
  (2025, 21, 1, '授予角色权限', 3, 'POST', '/system/roles/grant', '', 'system:role:grant', '', 5, 2, 1, 0, 0),
  (2026, 21, 1, '查看角色权限', 3, 'GET', '/system/roles/:id/grant', '', 'system:role:view', '', 6, 2, 1, 0, 0),
  (2027, 21, 1, '查看权限列表', 3, 'GET', '/system/perms', '', 'system:role:view', '', 7, 2, 1, 0, 0),

  (2031, 22, 1, '查看菜单树', 3, 'GET', '/system/menus/tree/:roleId', '', 'system:menu:view', '', 1, 2, 1, 0, 0),
  (2032, 22, 1, '查看菜单列表', 3, 'GET', '/system/menus', '', 'system:menu:view', '', 2, 2, 1, 0, 0),
  (2033, 22, 1, '创建菜单权限', 3, 'POST', '/system/menus', '', 'system:menu:add', '', 3, 2, 1, 0, 0),
  (2034, 22, 1, '修改菜单权限', 3, 'PUT', '/system/menus', '', 'system:menu:update', '', 4, 2, 1, 0, 0),
  (2035, 22, 1, '删除菜单权限', 3, 'DELETE', '/system/menus/:id', '', 'system:menu:delete', '', 5, 2, 1, 0, 0),

  (2041, 26, 1, '查看登录日志', 3, 'GET', '/system/logs/login', '', 'system:login-log:view', '', 1, 2, 1, 0, 0),
  (2042, 27, 1, '查看操作日志', 3, 'GET', '/system/logs/op', '', 'system:op-log:view', '', 1, 2, 1, 0, 0),

  (2051, 25, 1, '查看系统配置', 3, 'GET', '/system/configs', '', 'system:config:view', '', 1, 2, 1, 0, 0),
  (2052, 25, 1, '创建系统配置', 3, 'POST', '/system/configs', '', 'system:config:add', '', 2, 2, 1, 0, 0),
  (2053, 25, 1, '修改系统配置', 3, 'PUT', '/system/configs', '', 'system:config:update', '', 3, 2, 1, 0, 0),
  (2054, 25, 1, '删除系统配置', 3, 'DELETE', '/system/configs/:id', '', 'system:config:delete', '', 4, 2, 1, 0, 0),

  (2061, 23, 1, '查看租户列表', 3, 'GET', '/system/tenants', '', 'system:tenant:view', '', 1, 2, 1, 0, 0),
  (2062, 23, 1, '查看租户详情', 3, 'GET', '/system/tenant/detail', '', 'system:tenant:view', '', 2, 2, 1, 0, 0),
  (2063, 23, 1, '创建租户', 3, 'POST', '/system/tenants', '', 'system:tenant:add', '', 3, 2, 1, 0, 0),
  (2064, 23, 1, '修改租户', 3, 'PUT', '/system/tenants', '', 'system:tenant:update', '', 4, 2, 1, 0, 0),
  (2065, 23, 1, '删除租户', 3, 'DELETE', '/system/tenants/:id', '', 'system:tenant:delete', '', 5, 2, 1, 0, 0),

  (2071, 24, 1, '查看租户域名', 3, 'GET', '/system/tenant-domains', '', 'system:tenant-domain:view', '', 1, 2, 1, 0, 0),
  (2072, 24, 1, '创建租户域名', 3, 'POST', '/system/tenant-domains', '', 'system:tenant-domain:add', '', 2, 2, 1, 0, 0),
  (2073, 24, 1, '修改租户域名', 3, 'PUT', '/system/tenant-domains', '', 'system:tenant-domain:update', '', 3, 2, 1, 0, 0),
  (2074, 24, 1, '删除租户域名', 3, 'DELETE', '/system/tenant-domains/:id', '', 'system:tenant-domain:delete', '', 4, 2, 1, 0, 0);

-- owner：全部菜单和按钮权限。
INSERT INTO sys_role_menu (tenant_id, role_id, menu_id)
SELECT 1, 1, id FROM sys_menu;

-- admin：全部 Core 业务菜单和操作权限。
INSERT INTO sys_role_menu (tenant_id, role_id, menu_id)
SELECT 1, 2, id
FROM sys_menu
WHERE id = 1 OR parent_id = 1 OR id BETWEEN 1000 AND 1999;

-- viewer：业务菜单以及所有 Core GET 权限。
INSERT INTO sys_role_menu (tenant_id, role_id, menu_id)
SELECT 1, 3, id
FROM sys_menu
WHERE id = 1
   OR parent_id = 1
   OR (id BETWEEN 1000 AND 1999 AND method = 'GET');

-- 系统基础配置。枚举名称与 proto/system/enum.proto 保持一致。
INSERT INTO sys_config (
  id, tenant_id, config_key, config_value, remark, create_times, update_times
) VALUES
  (
    1, 0, 'SYSTEM_CORE',
    JSON_OBJECT(
      'site_name', 'AppForge',
      'site_logo', '',
      'is_captcha_enabled', 2,
      'is_register_enabled', 2,
      'is_guest_enabled', 2,
      'is_crypto_enabled', 2,
      'admin_must_google_f2a', 2,
      'app_must_google_f2a', 2
    ),
    '管理后台公共配置',
    UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3)) * 1000,
    UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3)) * 1000
  ),
  (
    2, 0, 'OBJECT_STORAGE',
    JSON_OBJECT(
      'oss_type', 3,
      'oss_domain', 'http://localhost:9000/appforge',
      'minio', JSON_OBJECT(
        'endpoint', 'minio:9000',
        'access_key_id', 'appforge',
        'access_key_secret', 'appforge_dev_minio',
        'bucket_name', 'appforge',
        'bucket_url', 'http://localhost:9000/appforge'
      )
    ),
    '开发环境 MinIO 配置',
    UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3)) * 1000,
    UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3)) * 1000
  );

