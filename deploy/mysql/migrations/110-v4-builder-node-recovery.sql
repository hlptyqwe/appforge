-- AppForge V4 Builder隔离节点人工恢复接口权限。
SET NAMES utf8mb4;
SET time_zone = '+08:00';

INSERT INTO sys_menu (
  id,parent_id,app_scope,name,menu_type,method,path,component,perms,
  icon,sort,visible,enabled,create_times,update_times
) VALUES
  (1080,16,1,'恢复Builder节点',3,'POST','/core/build-cluster/nodes/:id/recover','','core:build-cluster:recover','',10,2,1,0,0)
ON DUPLICATE KEY UPDATE name=VALUES(name),parent_id=VALUES(parent_id),app_scope=VALUES(app_scope),method=VALUES(method),
  path=VALUES(path),component=VALUES(component),perms=VALUES(perms),icon=VALUES(icon),sort=VALUES(sort),visible=VALUES(visible),enabled=VALUES(enabled);

INSERT IGNORE INTO sys_role_menu (tenant_id,role_id,menu_id)
SELECT r.tenant_id,r.id,1080 FROM sys_role r
WHERE r.app_scope=1 AND r.code IN ('owner','admin');

INSERT INTO sys_schema_migration (version,description)
VALUES ('20260814_110_v4_builder_node_recovery','V4 Builder隔离节点人工恢复接口权限')
ON DUPLICATE KEY UPDATE description=VALUES(description);
