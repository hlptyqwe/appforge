-- AppForge V5 Webhook管理与投递日志权限。
SET NAMES utf8mb4;
SET time_zone = '+08:00';

INSERT INTO sys_menu (
  id,parent_id,app_scope,name,menu_type,method,path,component,perms,
  icon,sort,visible,enabled,create_times,update_times
) VALUES
  (4114,4100,1,'创建Webhook',3,'POST','/core/developer/webhooks','','core:developer:webhook','',5,2,1,0,0),
  (4115,4100,1,'查看Webhook',3,'GET','/core/developer/webhooks','','core:developer:view','',6,2,1,0,0),
  (4116,4100,1,'查看Webhook详情',3,'GET','/core/developer/webhooks/:id','','core:developer:view','',7,2,1,0,0),
  (4117,4100,1,'更新Webhook',3,'PUT','/core/developer/webhooks/:id','','core:developer:webhook','',8,2,1,0,0),
  (4118,4100,1,'轮换Webhook密钥',3,'POST','/core/developer/webhooks/:id/rotate-secret','','core:developer:webhook','',9,2,1,0,0),
  (4119,4100,1,'测试Webhook',3,'POST','/core/developer/webhooks/:id/test','','core:developer:webhook','',10,2,1,0,0),
  (4120,4100,1,'查看Webhook投递',3,'GET','/core/developer/webhook-deliveries','','core:developer:view','',11,2,1,0,0),
  (4121,4100,1,'重放Webhook投递',3,'POST','/core/developer/webhook-deliveries/:id/replay','','core:developer:webhook','',12,2,1,0,0),
  (4214,4200,2,'创建Webhook',3,'POST','/core/developer/webhooks','','core:developer:webhook','',5,2,1,0,0),
  (4215,4200,2,'查看Webhook',3,'GET','/core/developer/webhooks','','core:developer:view','',6,2,1,0,0),
  (4216,4200,2,'查看Webhook详情',3,'GET','/core/developer/webhooks/:id','','core:developer:view','',7,2,1,0,0),
  (4217,4200,2,'更新Webhook',3,'PUT','/core/developer/webhooks/:id','','core:developer:webhook','',8,2,1,0,0),
  (4218,4200,2,'轮换Webhook密钥',3,'POST','/core/developer/webhooks/:id/rotate-secret','','core:developer:webhook','',9,2,1,0,0),
  (4219,4200,2,'测试Webhook',3,'POST','/core/developer/webhooks/:id/test','','core:developer:webhook','',10,2,1,0,0),
  (4220,4200,2,'查看Webhook投递',3,'GET','/core/developer/webhook-deliveries','','core:developer:view','',11,2,1,0,0),
  (4221,4200,2,'重放Webhook投递',3,'POST','/core/developer/webhook-deliveries/:id/replay','','core:developer:webhook','',12,2,1,0,0)
ON DUPLICATE KEY UPDATE name=VALUES(name),parent_id=VALUES(parent_id),app_scope=VALUES(app_scope),method=VALUES(method),
  path=VALUES(path),component=VALUES(component),perms=VALUES(perms),sort=VALUES(sort),visible=VALUES(visible),enabled=VALUES(enabled);

INSERT IGNORE INTO sys_role_menu (tenant_id,role_id,menu_id)
SELECT r.tenant_id,r.id,m.id FROM sys_role r JOIN sys_menu m ON m.app_scope=r.app_scope
WHERE r.app_scope=1 AND r.code IN ('owner','admin') AND m.id BETWEEN 4114 AND 4121;
INSERT IGNORE INTO sys_role_menu (tenant_id,role_id,menu_id)
SELECT r.tenant_id,r.id,m.id FROM sys_role r JOIN sys_menu m ON m.app_scope=r.app_scope
WHERE r.app_scope=1 AND r.code='viewer' AND m.id IN (4115,4116,4120);
INSERT IGNORE INTO sys_role_menu (tenant_id,role_id,menu_id)
SELECT r.tenant_id,r.id,m.id FROM sys_role r JOIN sys_menu m ON m.app_scope=r.app_scope
WHERE r.app_scope=2 AND r.code IN ('owner','admin') AND m.id BETWEEN 4214 AND 4221;
INSERT IGNORE INTO sys_role_menu (tenant_id,role_id,menu_id)
SELECT r.tenant_id,r.id,m.id FROM sys_role r JOIN sys_menu m ON m.app_scope=r.app_scope
WHERE r.app_scope=2 AND r.code='viewer' AND m.id IN (4215,4216,4220);

INSERT INTO sys_schema_migration (version,description)
VALUES ('20260814_102_v5_webhook_permissions','V5 Webhook管理和投递日志权限')
ON DUPLICATE KEY UPDATE description=VALUES(description);
