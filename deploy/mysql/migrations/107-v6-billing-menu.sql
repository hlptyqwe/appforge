-- AppForge V6 商业化菜单、代理端和管理端路由权限。
SET NAMES utf8mb4;
SET time_zone = '+08:00';

INSERT INTO sys_menu (
  id,parent_id,app_scope,name,menu_type,method,path,component,perms,
  icon,sort,visible,enabled,create_times,update_times
) VALUES
  (4300,1,1,'商业化管理',2,'','/platform/billing','platform/billing','','CreditCard',90,1,1,0,0),
  (4310,4300,1,'查看套餐',3,'GET','/core/billing/plans','','billing:view','',1,2,1,0,0),
  (4311,4300,1,'创建套餐版本',3,'POST','/core/billing/plans','','billing:plan:manage','',2,2,1,0,0),
  (4312,4300,1,'退役套餐版本',3,'POST','/core/billing/plans/:id/retire','','billing:plan:manage','',3,2,1,0,0),
  (4313,4300,1,'查看租户订阅',3,'GET','/core/billing/subscription','','billing:view','',4,2,1,0,0),
  (4314,4300,1,'维护人工合同',3,'POST','/core/billing/contracts','','billing:contract:manage','',5,2,1,0,0),
  (4315,4300,1,'变更订阅',3,'POST','/core/billing/subscription/change','','billing:subscription:manage','',6,2,1,0,0),
  (4316,4300,1,'取消订阅',3,'POST','/core/billing/subscription/cancel','','billing:subscription:manage','',7,2,1,0,0),
  (4317,4300,1,'查看用量',3,'GET','/core/billing/usage','','billing:view','',8,2,1,0,0),
  (4318,4300,1,'查看账单',3,'GET','/core/billing/invoices','','billing:view','',9,2,1,0,0),
  (4319,4300,1,'创建结账会话',3,'POST','/core/billing/checkout','','billing:subscription:manage','',10,2,1,0,0),
  (4400,3000,2,'套餐与账单',2,'','/platform/billing','platform/billing','','CreditCard',70,1,1,0,0),
  (4410,4400,2,'查看套餐',3,'GET','/core/billing/plans','','billing:view','',1,2,1,0,0),
  (4411,4400,2,'查看订阅',3,'GET','/core/billing/subscription','','billing:view','',2,2,1,0,0),
  (4412,4400,2,'在线结账',3,'POST','/core/billing/checkout','','billing:subscription:manage','',3,2,1,0,0),
  (4413,4400,2,'变更订阅',3,'POST','/core/billing/subscription/change','','billing:subscription:manage','',4,2,1,0,0),
  (4414,4400,2,'取消订阅',3,'POST','/core/billing/subscription/cancel','','billing:subscription:manage','',5,2,1,0,0),
  (4415,4400,2,'查看用量',3,'GET','/core/billing/usage','','billing:view','',6,2,1,0,0),
  (4416,4400,2,'查看账单',3,'GET','/core/billing/invoices','','billing:view','',7,2,1,0,0)
ON DUPLICATE KEY UPDATE name=VALUES(name),parent_id=VALUES(parent_id),app_scope=VALUES(app_scope),method=VALUES(method),
  path=VALUES(path),component=VALUES(component),perms=VALUES(perms),icon=VALUES(icon),sort=VALUES(sort),visible=VALUES(visible),enabled=VALUES(enabled);

INSERT IGNORE INTO sys_role_menu (tenant_id,role_id,menu_id)
SELECT r.tenant_id,r.id,m.id FROM sys_role r JOIN sys_menu m ON m.app_scope=r.app_scope
WHERE r.app_scope=1 AND r.code IN ('owner','admin') AND m.id BETWEEN 4300 AND 4319;
INSERT IGNORE INTO sys_role_menu (tenant_id,role_id,menu_id)
SELECT r.tenant_id,r.id,m.id FROM sys_role r JOIN sys_menu m ON m.app_scope=r.app_scope
WHERE r.app_scope=1 AND r.code='viewer' AND m.id IN (4300,4310,4313,4317,4318);
INSERT IGNORE INTO sys_role_menu (tenant_id,role_id,menu_id)
SELECT r.tenant_id,r.id,m.id FROM sys_role r JOIN sys_menu m ON m.app_scope=r.app_scope
WHERE r.app_scope=2 AND r.code IN ('owner','admin') AND m.id BETWEEN 4400 AND 4416;
INSERT IGNORE INTO sys_role_menu (tenant_id,role_id,menu_id)
SELECT r.tenant_id,r.id,m.id FROM sys_role r JOIN sys_menu m ON m.app_scope=r.app_scope
WHERE r.app_scope=2 AND r.code='viewer' AND m.id IN (4400,4410,4411,4415,4416);

INSERT INTO sys_schema_migration (version,description)
VALUES ('20260814_107_v6_billing_menu','V6商业化管理和代理端套餐账单菜单权限')
ON DUPLICATE KEY UPDATE description=VALUES(description);
