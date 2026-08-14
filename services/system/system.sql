-- =============================
-- 平台管理用户（统一用户表）
-- =============================
DROP TABLE IF EXISTS sys_user;
CREATE TABLE sys_user (
  id BIGINT AUTO_INCREMENT COMMENT '用户ID',

  tenant_id BIGINT NOT NULL DEFAULT 0 COMMENT '所属租户ID：0=系统侧，>0=租户ID',
  app_scope TINYINT NOT NULL DEFAULT 1 COMMENT '应用范围：1平台管理端 2代理端',
  user_type TINYINT NOT NULL DEFAULT 1 COMMENT '账号类型：1系统管理员 2租户所有者 3租户成员',
  is_owner TINYINT NOT NULL DEFAULT 2 COMMENT '是否租户所有者：1是 2否',

  username VARCHAR(64) NOT NULL DEFAULT '' COMMENT '登录账号',
  password VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'bcrypt密码',

  nickname VARCHAR(64) DEFAULT '' COMMENT '昵称',
  avatar VARCHAR(255) DEFAULT '' COMMENT '头像',

  enabled TINYINT DEFAULT 1 COMMENT '启用开关：1启用 2禁用',

  -- google 2fa
  google_secret VARCHAR(255) DEFAULT '' COMMENT '2FA secret(加密存储)',
  google_enabled TINYINT DEFAULT 2 COMMENT 'Google2FA开关：1启用 2禁用',

  perms_ver INT DEFAULT 1 COMMENT '权限版本(角色变化强制token失效)',

  last_login_ip VARCHAR(64) DEFAULT '' COMMENT '最后登录IP',
  last_login_at BIGINT NOT NULL DEFAULT 0 COMMENT '最后登录时间',

  create_by BIGINT NOT NULL DEFAULT 0 COMMENT '创建人ID',
  create_times BIGINT NOT NULL DEFAULT 0 COMMENT '创建时间（Unix毫秒）',
  update_times BIGINT NOT NULL DEFAULT 0 COMMENT '更新时间（Unix毫秒）',

  PRIMARY KEY (id),
  UNIQUE KEY uk_tenant_scope_username (tenant_id, app_scope, username),
  UNIQUE KEY uk_scope_username (app_scope, username) COMMENT '同一应用端登录账号全局唯一，避免跨租户登录歧义',
  INDEX idx_tenant_id(tenant_id),
  INDEX idx_app_scope(app_scope),
  INDEX idx_user_type(user_type),
  INDEX idx_owner(is_owner),
  INDEX idx_enabled(enabled)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='统一用户表';

-- =============================
-- 角色
-- 说明：
-- 1. tenant_id = 0  -> 系统角色
-- 2. tenant_id > 0  -> 某个租户自己的角色
-- =============================
DROP TABLE IF EXISTS sys_role;
CREATE TABLE sys_role (
  id BIGINT AUTO_INCREMENT COMMENT '角色ID',

  tenant_id BIGINT NOT NULL DEFAULT 0 COMMENT '所属租户ID：0=系统角色，>0=租户角色',
  app_scope TINYINT NOT NULL DEFAULT 1 COMMENT '应用范围：1平台管理端 2代理端',

  name VARCHAR(64) NOT NULL DEFAULT '' COMMENT '角色名称',
  code VARCHAR(64) NOT NULL DEFAULT '' COMMENT '角色标识(如admin)',

  enabled TINYINT DEFAULT 1 COMMENT '启用开关：1启用 2禁用',

  remark VARCHAR(255) DEFAULT '' COMMENT '角色说明',

  create_times BIGINT NOT NULL DEFAULT 0 COMMENT '创建时间（Unix毫秒）',
  update_times BIGINT NOT NULL DEFAULT 0 COMMENT '更新时间（Unix毫秒）',

  PRIMARY KEY (id),
  UNIQUE KEY uk_tenant_scope_role_name(tenant_id, app_scope, name),
  UNIQUE KEY uk_tenant_scope_role_code(tenant_id, app_scope, code),
  INDEX idx_tenant_id(tenant_id),
  INDEX idx_app_scope(app_scope),
  INDEX idx_enabled(enabled)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='角色表';


-- =============================
-- 用户-角色
-- =============================
DROP TABLE IF EXISTS sys_user_role;
CREATE TABLE sys_user_role (
  id BIGINT AUTO_INCREMENT COMMENT '主键ID',

  tenant_id BIGINT NOT NULL DEFAULT 0 COMMENT '所属租户ID：0=系统侧，>0=租户ID',
  user_id BIGINT NOT NULL DEFAULT 0 COMMENT '用户ID',
  role_id BIGINT NOT NULL DEFAULT 0 COMMENT '角色ID',

  PRIMARY KEY (id),
  UNIQUE KEY uk_user_role(user_id, role_id),
  INDEX idx_tenant_id(tenant_id),
  INDEX idx_user(user_id),
  INDEX idx_role(role_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户角色关联';


-- =============================
-- 菜单/按钮（核心RBAC）
-- 说明：
-- 1. app_scope = 1 -> 平台管理端菜单
-- 2. app_scope = 2 -> 代理端菜单
-- 3. 系统端与租户端通过tenant_id和角色权限进行隔离
-- =============================
DROP TABLE IF EXISTS sys_menu;
CREATE TABLE sys_menu (
  id BIGINT AUTO_INCREMENT COMMENT '菜单ID',

  parent_id BIGINT DEFAULT 0 COMMENT '父级ID',
  app_scope TINYINT NOT NULL DEFAULT 1 COMMENT '应用范围：1平台管理端 2代理端',

  name VARCHAR(64) NOT NULL DEFAULT '' COMMENT '名称',

  menu_type TINYINT NOT NULL DEFAULT 0 COMMENT '菜单类型：0未知 1目录 2菜单 3按钮',

  method VARCHAR(16) DEFAULT '' COMMENT '请求方法 GET POST PUT DELETE',
  path VARCHAR(255) DEFAULT '' COMMENT '路由路径',
  component VARCHAR(255) DEFAULT '' COMMENT '前端组件',

  perms VARCHAR(128) DEFAULT '' COMMENT '按钮权限标识 sys:user:add',

  icon VARCHAR(64) DEFAULT '' COMMENT '菜单图标',
  sort INT DEFAULT 0 COMMENT '排序值，值越小越靠前',

  visible TINYINT DEFAULT 1 COMMENT '显示开关：1显示 2隐藏',
  enabled TINYINT DEFAULT 1 COMMENT '启用开关：1启用 2禁用',

  create_times BIGINT NOT NULL DEFAULT 0 COMMENT '创建时间（Unix毫秒）',
  update_times BIGINT NOT NULL DEFAULT 0 COMMENT '更新时间（Unix毫秒）',

  PRIMARY KEY (id),
  INDEX idx_parent(parent_id),
  INDEX idx_app_scope(app_scope),
  INDEX idx_perms(perms)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='菜单权限';


-- =============================
-- 角色-菜单权限
-- =============================
DROP TABLE IF EXISTS sys_role_menu;
CREATE TABLE sys_role_menu (
  id BIGINT AUTO_INCREMENT COMMENT '主键ID',

  tenant_id BIGINT NOT NULL DEFAULT 0 COMMENT '所属租户ID：0=系统侧，>0=租户ID',
  role_id BIGINT NOT NULL DEFAULT 0 COMMENT '角色ID',
  menu_id BIGINT NOT NULL DEFAULT 0 COMMENT '菜单ID',

  PRIMARY KEY (id),
  UNIQUE KEY uk_role_menu(role_id, menu_id),
  INDEX idx_tenant_id(tenant_id),
  INDEX idx_role(role_id),
  INDEX idx_menu(menu_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='角色菜单权限';


-- =============================
-- 登录日志
-- 说明：
-- 增加 tenant_id，方便系统侧/租户侧隔离查询
-- =============================
DROP TABLE IF EXISTS sys_login_log;
CREATE TABLE sys_login_log (
  id BIGINT AUTO_INCREMENT COMMENT '登录日志ID',

  tenant_id BIGINT NOT NULL DEFAULT 0 COMMENT '所属租户ID：0=系统侧，>0=租户ID',

  user_id BIGINT COMMENT '用户ID',
  username VARCHAR(64) COMMENT '登录账号',

  ip VARCHAR(64),
  ua VARCHAR(255),

  success TINYINT COMMENT '1成功 0失败',
  msg VARCHAR(255),

  login_at BIGINT NOT NULL DEFAULT 0 COMMENT '登录时间（Unix毫秒）',
  
  PRIMARY KEY (id),
  INDEX idx_tenant_id(tenant_id),
  INDEX idx_user(user_id),
  INDEX idx_time(login_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='登录日志';


-- =============================
-- 操作日志
-- 说明：
-- 增加 tenant_id，方便系统侧/租户侧隔离查询
-- =============================
DROP TABLE IF EXISTS sys_op_log;

CREATE TABLE sys_op_log (
  id BIGINT AUTO_INCREMENT COMMENT '操作日志ID',

  tenant_id BIGINT NOT NULL DEFAULT 0 COMMENT '所属租户ID：0=系统侧，>0=租户ID',

  user_id BIGINT DEFAULT 0 COMMENT '操作人ID',
  username VARCHAR(64) DEFAULT '' COMMENT '操作人账号',

  module VARCHAR(64) DEFAULT '' COMMENT '模块',
  action VARCHAR(64) DEFAULT '' COMMENT '操作',

  method VARCHAR(16) DEFAULT '' COMMENT '请求方法',
  path VARCHAR(255) DEFAULT '' COMMENT '请求路径',

  req TEXT COMMENT '请求参数',
  resp TEXT COMMENT '响应内容',

  ip VARCHAR(64) DEFAULT '' COMMENT 'IP',

  cost_ms INT DEFAULT 0 COMMENT '耗时',

  create_times BIGINT NOT NULL DEFAULT 0 COMMENT '创建时间（Unix毫秒）',
  update_times BIGINT NOT NULL DEFAULT 0 COMMENT '更新时间（Unix毫秒）',

  PRIMARY KEY (id),
  INDEX idx_tenant_id(tenant_id),
  INDEX idx_user(user_id),
  INDEX idx_time(create_times),
  INDEX idx_path(path)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='操作日志';


-- =============================
-- 系统配置（系统级）
-- 说明：
-- 这张表保留为系统配置，不做租户隔离
-- 如需租户配置，建议单独增加 tenant_config
-- =============================
DROP TABLE IF EXISTS sys_config;
CREATE TABLE sys_config (
  id BIGINT AUTO_INCREMENT COMMENT '配置ID',
  tenant_id BIGINT NOT NULL DEFAULT 0 COMMENT '所属租户ID：0=系统侧，>0=租户ID',
  config_key VARCHAR(64) COMMENT '配置键',
  config_value JSON COMMENT '配置值',
  remark VARCHAR(255) COMMENT '配置说明',

  create_times BIGINT NOT NULL DEFAULT 0 COMMENT '创建时间（Unix毫秒）',
  update_times BIGINT NOT NULL DEFAULT 0 COMMENT '更新时间（Unix毫秒）',

  PRIMARY KEY (id),
  UNIQUE KEY uk_config_key(tenant_id, config_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='系统配置';

-- =============================
-- =============================
-- 租户表
-- 说明：
-- 这里只保留租户资料，不再存登录账号密码
-- 租户主账号统一存到 sys_user
-- =============================
DROP TABLE IF EXISTS sys_tenant;
CREATE TABLE sys_tenant (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '租户ID',
  tenant_code VARCHAR(64) NOT NULL DEFAULT '' COMMENT '租户编码',
  tenant_name VARCHAR(128) NOT NULL DEFAULT '' COMMENT '租户名称',
  enabled TINYINT NOT NULL DEFAULT 1 COMMENT '启用开关：1启用 2禁用',
  expire_time BIGINT DEFAULT 0 COMMENT '到期时间',
  contact_name VARCHAR(64) DEFAULT NULL COMMENT '联系人',
  contact_phone VARCHAR(32) DEFAULT NULL COMMENT '联系电话',
  login_ip VARCHAR(64) DEFAULT NULL COMMENT '最后登录IP',
  login_time BIGINT DEFAULT 0 COMMENT '最后登录时间',
  login_count INT DEFAULT 0 COMMENT '登录次数',
  remark VARCHAR(255) DEFAULT NULL COMMENT '备注',
  create_by VARCHAR(64) DEFAULT NULL COMMENT '创建人',
  create_times BIGINT NOT NULL DEFAULT 0 COMMENT '创建时间',
  update_by VARCHAR(64) DEFAULT NULL COMMENT '更新人',
  update_times BIGINT NOT NULL DEFAULT 0 COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_tenant_code (tenant_code),
  KEY idx_enabled (enabled),
  KEY idx_expire_time (expire_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='租户表';

DROP TABLE IF EXISTS sys_tenant_domain;
CREATE TABLE sys_tenant_domain (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  tenant_id BIGINT NOT NULL COMMENT '租户ID',
  origin VARCHAR(255) NOT NULL COMMENT '规范化域名Origin',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '1使用中 2已退役 3已禁用',
  priority INT NOT NULL DEFAULT 0 COMMENT '使用中域名跳转优先级',
  create_times BIGINT NOT NULL DEFAULT 0 COMMENT '创建时间（Unix毫秒）',
  update_times BIGINT NOT NULL DEFAULT 0 COMMENT '更新时间（Unix毫秒）',
  PRIMARY KEY (id),
  UNIQUE KEY uk_tenant_origin (tenant_id, origin),
  KEY idx_tenant_status_priority (tenant_id, status, priority)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='租户业务域名注册表';

-- Core 业务表已移动到 services/core/core.sql。
