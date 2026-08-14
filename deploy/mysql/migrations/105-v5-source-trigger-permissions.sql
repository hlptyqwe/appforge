-- AppForge V5 源码平台预定义触发策略和入站事件权限。
SET NAMES utf8mb4;
SET time_zone = '+08:00';

INSERT INTO sys_menu (
  id,parent_id,app_scope,name,menu_type,method,path,component,perms,
  icon,sort,visible,enabled,create_times,update_times
) VALUES
  (4131,4100,1,'创建源码构建触发策略',3,'POST','/core/developer/source-build-triggers','','core:developer:source-trigger','',22,2,1,0,0),
  (4132,4100,1,'查看源码构建触发策略',3,'GET','/core/developer/source-build-triggers','','core:developer:view','',23,2,1,0,0),
  (4133,4100,1,'查看源码构建触发策略详情',3,'GET','/core/developer/source-build-triggers/:id','','core:developer:view','',24,2,1,0,0),
  (4134,4100,1,'更新源码构建触发策略',3,'PUT','/core/developer/source-build-triggers/:id','','core:developer:source-trigger','',25,2,1,0,0),
  (4135,4100,1,'轮换源码Webhook密钥',3,'POST','/core/developer/source-build-triggers/:id/rotate-secret','','core:developer:source-trigger','',26,2,1,0,0),
  (4136,4100,1,'查看源码Webhook事件',3,'GET','/core/developer/source-webhook-events','','core:developer:view','',27,2,1,0,0),
  (4231,4200,2,'创建源码构建触发策略',3,'POST','/core/developer/source-build-triggers','','core:developer:source-trigger','',22,2,1,0,0),
  (4232,4200,2,'查看源码构建触发策略',3,'GET','/core/developer/source-build-triggers','','core:developer:view','',23,2,1,0,0),
  (4233,4200,2,'查看源码构建触发策略详情',3,'GET','/core/developer/source-build-triggers/:id','','core:developer:view','',24,2,1,0,0),
  (4234,4200,2,'更新源码构建触发策略',3,'PUT','/core/developer/source-build-triggers/:id','','core:developer:source-trigger','',25,2,1,0,0),
  (4235,4200,2,'轮换源码Webhook密钥',3,'POST','/core/developer/source-build-triggers/:id/rotate-secret','','core:developer:source-trigger','',26,2,1,0,0),
  (4236,4200,2,'查看源码Webhook事件',3,'GET','/core/developer/source-webhook-events','','core:developer:view','',27,2,1,0,0)
ON DUPLICATE KEY UPDATE name=VALUES(name),parent_id=VALUES(parent_id),app_scope=VALUES(app_scope),method=VALUES(method),
  path=VALUES(path),component=VALUES(component),perms=VALUES(perms),sort=VALUES(sort),visible=VALUES(visible),enabled=VALUES(enabled);

INSERT IGNORE INTO sys_role_menu (tenant_id,role_id,menu_id)
SELECT r.tenant_id,r.id,m.id FROM sys_role r JOIN sys_menu m ON m.app_scope=r.app_scope
WHERE r.app_scope=1 AND r.code IN ('owner','admin') AND m.id BETWEEN 4131 AND 4136;
INSERT IGNORE INTO sys_role_menu (tenant_id,role_id,menu_id)
SELECT r.tenant_id,r.id,m.id FROM sys_role r JOIN sys_menu m ON m.app_scope=r.app_scope
WHERE r.app_scope=1 AND r.code='viewer' AND m.id IN (4132,4133,4136);
INSERT IGNORE INTO sys_role_menu (tenant_id,role_id,menu_id)
SELECT r.tenant_id,r.id,m.id FROM sys_role r JOIN sys_menu m ON m.app_scope=r.app_scope
WHERE r.app_scope=2 AND r.code IN ('owner','admin') AND m.id BETWEEN 4231 AND 4236;
INSERT IGNORE INTO sys_role_menu (tenant_id,role_id,menu_id)
SELECT r.tenant_id,r.id,m.id FROM sys_role r JOIN sys_menu m ON m.app_scope=r.app_scope
WHERE r.app_scope=2 AND r.code='viewer' AND m.id IN (4232,4233,4236);

INSERT INTO sys_schema_migration (version,description)
VALUES ('20260814_105_v5_source_trigger_permissions','V5源码平台构建触发策略和入站事件权限')
ON DUPLICATE KEY UPDATE description=VALUES(description);
