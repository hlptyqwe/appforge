-- AppForge V7 企业部署健康、版本、许可证和升级诊断只读页面。
SET NAMES utf8mb4;
SET time_zone = '+08:00';

INSERT INTO sys_menu (
  id,parent_id,app_scope,name,menu_type,method,path,component,perms,
  icon,sort,visible,enabled,create_times,update_times
) VALUES
  (4700,2,1,'部署与升级',2,'','/platform/deployment','platform/deployment','','Monitor',90,1,1,0,0),
  (4710,4700,1,'查看企业部署状态',3,'GET','/core/enterprise/deployment','','enterprise:deployment:view','',1,2,1,0,0)
ON DUPLICATE KEY UPDATE name=VALUES(name),parent_id=VALUES(parent_id),app_scope=VALUES(app_scope),method=VALUES(method),
  path=VALUES(path),component=VALUES(component),perms=VALUES(perms),icon=VALUES(icon),sort=VALUES(sort),visible=VALUES(visible),enabled=VALUES(enabled);

-- 部署与许可证属于平台级信息；按实际平台用户关联角色，避免依赖历史角色tenant_id数据。
DELETE rm FROM sys_role_menu rm
WHERE rm.menu_id IN (4700,4710)
  AND NOT EXISTS (
    SELECT 1 FROM sys_user_role ur
    JOIN sys_user u ON u.id=ur.user_id
    WHERE ur.role_id=rm.role_id AND u.tenant_id=0 AND u.app_scope=1 AND u.user_type=1
  );

INSERT IGNORE INTO sys_role_menu (tenant_id,role_id,menu_id)
SELECT DISTINCT 0,r.id,m.id
FROM sys_role r
JOIN sys_user_role ur ON ur.role_id=r.id
JOIN sys_user u ON u.id=ur.user_id AND u.tenant_id=0 AND u.app_scope=1 AND u.user_type=1
JOIN sys_menu m ON m.app_scope=r.app_scope
WHERE r.app_scope=1 AND r.code IN ('owner','admin','viewer') AND m.id IN (4700,4710);

INSERT INTO sys_schema_migration (version,description)
VALUES ('20260815_111_v7_deployment_console','V7企业部署健康版本许可证升级诊断只读页面')
ON DUPLICATE KEY UPDATE description=VALUES(description);
