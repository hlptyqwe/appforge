-- AppForge V5 开发者中心菜单和API凭证权限。
SET NAMES utf8mb4;
SET time_zone = '+08:00';

DELETE rm FROM sys_role_menu rm
JOIN sys_menu m ON m.id=rm.menu_id
WHERE m.id IN (1080,3260) AND m.path='/core/developer/credentials';
DELETE FROM sys_menu WHERE id IN (1080,3260) AND path='/core/developer/credentials';

INSERT INTO sys_menu (
  id,parent_id,app_scope,name,menu_type,method,path,component,perms,
  icon,sort,visible,enabled,create_times,update_times
) VALUES
  (4100,1,1,'开发者中心',2,'','/platform/developer','platform/developer','','Connection',80,1,1,0,0),
  (4110,4100,1,'查看API凭证',3,'GET','/core/developer/credentials','','core:developer:view','',1,2,1,0,0),
  (4111,4100,1,'创建API凭证',3,'POST','/core/developer/credentials','','core:developer:credential','',2,2,1,0,0),
  (4112,4100,1,'轮换API凭证',3,'POST','/core/developer/credentials/:id/rotate','','core:developer:credential','',3,2,1,0,0),
  (4113,4100,1,'吊销API凭证',3,'POST','/core/developer/credentials/:id/revoke','','core:developer:credential','',4,2,1,0,0),
  (4200,3000,2,'开发者中心',2,'','/platform/developer','platform/developer','','Connection',60,1,1,0,0),
  (4210,4200,2,'查看API凭证',3,'GET','/core/developer/credentials','','core:developer:view','',1,2,1,0,0),
  (4211,4200,2,'创建API凭证',3,'POST','/core/developer/credentials','','core:developer:credential','',2,2,1,0,0),
  (4212,4200,2,'轮换API凭证',3,'POST','/core/developer/credentials/:id/rotate','','core:developer:credential','',3,2,1,0,0),
  (4213,4200,2,'吊销API凭证',3,'POST','/core/developer/credentials/:id/revoke','','core:developer:credential','',4,2,1,0,0)
ON DUPLICATE KEY UPDATE name=VALUES(name),parent_id=VALUES(parent_id),app_scope=VALUES(app_scope),method=VALUES(method),
  path=VALUES(path),component=VALUES(component),perms=VALUES(perms),icon=VALUES(icon),sort=VALUES(sort),visible=VALUES(visible),enabled=VALUES(enabled);

INSERT IGNORE INTO sys_role_menu (tenant_id,role_id,menu_id)
SELECT r.tenant_id,r.id,m.id FROM sys_role r JOIN sys_menu m ON m.app_scope=r.app_scope
WHERE r.app_scope=1 AND r.code IN ('owner','admin') AND m.id IN (4100,4110,4111,4112,4113);

INSERT IGNORE INTO sys_role_menu (tenant_id,role_id,menu_id)
SELECT r.tenant_id,r.id,m.id FROM sys_role r JOIN sys_menu m ON m.app_scope=r.app_scope
WHERE r.app_scope=1 AND r.code='viewer' AND m.id IN (4100,4110);

INSERT IGNORE INTO sys_role_menu (tenant_id,role_id,menu_id)
SELECT r.tenant_id,r.id,m.id FROM sys_role r JOIN sys_menu m ON m.app_scope=r.app_scope
WHERE r.app_scope=2 AND r.code IN ('owner','admin') AND m.id IN (4200,4210,4211,4212,4213);

INSERT IGNORE INTO sys_role_menu (tenant_id,role_id,menu_id)
SELECT r.tenant_id,r.id,m.id FROM sys_role r JOIN sys_menu m ON m.app_scope=r.app_scope
WHERE r.app_scope=2 AND r.code='viewer' AND m.id IN (4200,4210);

INSERT INTO sys_schema_migration (version,description)
VALUES ('20260814_101_v5_developer_menu','V5开发者中心菜单与API凭证权限')
ON DUPLICATE KEY UPDATE description=VALUES(description);
