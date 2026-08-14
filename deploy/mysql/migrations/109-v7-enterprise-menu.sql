-- AppForge V7 企业交付 Local Agent 管理菜单与接口权限。
SET NAMES utf8mb4;
SET time_zone = '+08:00';

INSERT INTO sys_menu (
  id,parent_id,app_scope,name,menu_type,method,path,component,perms,
  icon,sort,visible,enabled,create_times,update_times
) VALUES
  (4500,1,1,'本地构建节点',2,'','/platform/local-agents','platform/local-agents','','Connection',100,1,1,0,0),
  (4510,4500,1,'创建注册令牌',3,'POST','/core/enterprise/local-agents','','enterprise:agent:create','',1,2,1,0,0),
  (4511,4500,1,'查看本地节点',3,'GET','/core/enterprise/local-agents','','enterprise:agent:view','',2,2,1,0,0),
  (4512,4500,1,'查看本地节点详情',3,'GET','/core/enterprise/local-agents/:id','','enterprise:agent:view','',3,2,1,0,0),
  (4513,4500,1,'排空本地节点',3,'POST','/core/enterprise/local-agents/:id/drain','','enterprise:agent:manage','',4,2,1,0,0),
  (4514,4500,1,'吊销本地节点',3,'POST','/core/enterprise/local-agents/:id/revoke','','enterprise:agent:manage','',5,2,1,0,0),
  (4600,3000,2,'本地构建节点',2,'','/platform/local-agents','platform/local-agents','','Connection',80,1,1,0,0),
  (4610,4600,2,'创建注册令牌',3,'POST','/core/enterprise/local-agents','','enterprise:agent:create','',1,2,1,0,0),
  (4611,4600,2,'查看本地节点',3,'GET','/core/enterprise/local-agents','','enterprise:agent:view','',2,2,1,0,0),
  (4612,4600,2,'查看本地节点详情',3,'GET','/core/enterprise/local-agents/:id','','enterprise:agent:view','',3,2,1,0,0),
  (4613,4600,2,'排空本地节点',3,'POST','/core/enterprise/local-agents/:id/drain','','enterprise:agent:manage','',4,2,1,0,0),
  (4614,4600,2,'吊销本地节点',3,'POST','/core/enterprise/local-agents/:id/revoke','','enterprise:agent:manage','',5,2,1,0,0)
ON DUPLICATE KEY UPDATE name=VALUES(name),parent_id=VALUES(parent_id),app_scope=VALUES(app_scope),method=VALUES(method),
  path=VALUES(path),component=VALUES(component),perms=VALUES(perms),icon=VALUES(icon),sort=VALUES(sort),visible=VALUES(visible),enabled=VALUES(enabled);

INSERT IGNORE INTO sys_role_menu (tenant_id,role_id,menu_id)
SELECT r.tenant_id,r.id,m.id FROM sys_role r JOIN sys_menu m ON m.app_scope=r.app_scope
WHERE r.app_scope=1 AND r.code IN ('owner','admin') AND m.id BETWEEN 4500 AND 4514;
INSERT IGNORE INTO sys_role_menu (tenant_id,role_id,menu_id)
SELECT r.tenant_id,r.id,m.id FROM sys_role r JOIN sys_menu m ON m.app_scope=r.app_scope
WHERE r.app_scope=1 AND r.code='viewer' AND m.id IN (4500,4511,4512);
INSERT IGNORE INTO sys_role_menu (tenant_id,role_id,menu_id)
SELECT r.tenant_id,r.id,m.id FROM sys_role r JOIN sys_menu m ON m.app_scope=r.app_scope
WHERE r.app_scope=2 AND r.code IN ('owner','admin') AND m.id BETWEEN 4600 AND 4614;
INSERT IGNORE INTO sys_role_menu (tenant_id,role_id,menu_id)
SELECT r.tenant_id,r.id,m.id FROM sys_role r JOIN sys_menu m ON m.app_scope=r.app_scope
WHERE r.app_scope=2 AND r.code='viewer' AND m.id IN (4600,4611,4612);

INSERT INTO sys_schema_migration (version,description)
VALUES ('20260814_109_v7_enterprise_menu','V7企业交付Local Agent管理菜单和接口权限')
ON DUPLICATE KEY UPDATE description=VALUES(description);
