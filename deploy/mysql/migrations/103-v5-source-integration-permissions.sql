-- AppForge V5 GitHub/GitLab受控集成与授权仓库权限。
SET NAMES utf8mb4;
SET time_zone = '+08:00';

INSERT INTO sys_menu (
  id,parent_id,app_scope,name,menu_type,method,path,component,perms,
  icon,sort,visible,enabled,create_times,update_times
) VALUES
  (4122,4100,1,'查看代码平台集成',3,'GET','/core/developer/source-integrations','','core:developer:view','',13,2,1,0,0),
  (4123,4100,1,'查看代码平台集成详情',3,'GET','/core/developer/source-integrations/:id','','core:developer:view','',14,2,1,0,0),
  (4124,4100,1,'断开代码平台集成',3,'POST','/core/developer/source-integrations/:id/disconnect','','core:developer:source','',15,2,1,0,0),
  (4125,4100,1,'查看授权仓库',3,'GET','/core/developer/source-repositories','','core:developer:view','',16,2,1,0,0),
  (4126,4100,1,'撤销仓库授权',3,'POST','/core/developer/source-repositories/:id/revoke','','core:developer:source','',17,2,1,0,0),
  (4127,4100,1,'连接代码平台',3,'POST','/core/developer/source-integrations/:platform/authorize','','core:developer:source','',18,2,1,0,0),
  (4128,4100,1,'查看供应商可用仓库',3,'GET','/core/developer/source-integrations/:id/available-repositories','','core:developer:source','',19,2,1,0,0),
  (4129,4100,1,'授权供应商仓库',3,'POST','/core/developer/source-integrations/:id/repositories/:repositoryId/authorize','','core:developer:source','',20,2,1,0,0),
  (4130,4100,1,'导入供应商APK Artifact',3,'POST','/core/developer/source-artifacts/import','','core:developer:source','',21,2,1,0,0),
  (4222,4200,2,'查看代码平台集成',3,'GET','/core/developer/source-integrations','','core:developer:view','',13,2,1,0,0),
  (4223,4200,2,'查看代码平台集成详情',3,'GET','/core/developer/source-integrations/:id','','core:developer:view','',14,2,1,0,0),
  (4224,4200,2,'断开代码平台集成',3,'POST','/core/developer/source-integrations/:id/disconnect','','core:developer:source','',15,2,1,0,0),
  (4225,4200,2,'查看授权仓库',3,'GET','/core/developer/source-repositories','','core:developer:view','',16,2,1,0,0),
  (4226,4200,2,'撤销仓库授权',3,'POST','/core/developer/source-repositories/:id/revoke','','core:developer:source','',17,2,1,0,0),
  (4227,4200,2,'连接代码平台',3,'POST','/core/developer/source-integrations/:platform/authorize','','core:developer:source','',18,2,1,0,0),
  (4228,4200,2,'查看供应商可用仓库',3,'GET','/core/developer/source-integrations/:id/available-repositories','','core:developer:source','',19,2,1,0,0),
  (4229,4200,2,'授权供应商仓库',3,'POST','/core/developer/source-integrations/:id/repositories/:repositoryId/authorize','','core:developer:source','',20,2,1,0,0),
  (4230,4200,2,'导入供应商APK Artifact',3,'POST','/core/developer/source-artifacts/import','','core:developer:source','',21,2,1,0,0)
ON DUPLICATE KEY UPDATE name=VALUES(name),parent_id=VALUES(parent_id),app_scope=VALUES(app_scope),method=VALUES(method),
  path=VALUES(path),component=VALUES(component),perms=VALUES(perms),sort=VALUES(sort),visible=VALUES(visible),enabled=VALUES(enabled);

INSERT IGNORE INTO sys_role_menu (tenant_id,role_id,menu_id)
SELECT r.tenant_id,r.id,m.id FROM sys_role r JOIN sys_menu m ON m.app_scope=r.app_scope
WHERE r.app_scope=1 AND r.code IN ('owner','admin') AND m.id BETWEEN 4122 AND 4130;
INSERT IGNORE INTO sys_role_menu (tenant_id,role_id,menu_id)
SELECT r.tenant_id,r.id,m.id FROM sys_role r JOIN sys_menu m ON m.app_scope=r.app_scope
WHERE r.app_scope=1 AND r.code='viewer' AND m.id IN (4122,4123,4125);
INSERT IGNORE INTO sys_role_menu (tenant_id,role_id,menu_id)
SELECT r.tenant_id,r.id,m.id FROM sys_role r JOIN sys_menu m ON m.app_scope=r.app_scope
WHERE r.app_scope=2 AND r.code IN ('owner','admin') AND m.id BETWEEN 4222 AND 4230;
INSERT IGNORE INTO sys_role_menu (tenant_id,role_id,menu_id)
SELECT r.tenant_id,r.id,m.id FROM sys_role r JOIN sys_menu m ON m.app_scope=r.app_scope
WHERE r.app_scope=2 AND r.code='viewer' AND m.id IN (4222,4223,4225);

INSERT INTO sys_schema_migration (version,description)
VALUES ('20260814_103_v5_source_integration_permissions','V5 GitHub和GitLab受控集成权限')
ON DUPLICATE KEY UPDATE description=VALUES(description);
