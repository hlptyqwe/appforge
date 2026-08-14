# AppForge 开发环境部署

本目录提供可直接启动的本地开发环境，包含：

- MySQL 8.4：初始化 `system` 与 `core` 表结构和开发基础数据
- Redis 7.4：模型缓存与权限缓存
- etcd 3.6：初始化 common、system、core、builder、admin-api 配置
- MinIO：APK、keystore 和构建产物的开发对象存储
- system RPC、core RPC、builder RPC、常驻 builder worker、统一 API
- 平台管理端 `admin-ui` 与租户代理端 `agent-ui`

## 启动

需要 Docker 和 Docker Compose。首次启动会构建 Go 与 UI 镜像，并自动初始化数据库：

```bash
cd /Users/sky/local/go/src/appforge/deploy
make up
make ps
```

服务入口：

| 服务 | 地址 |
| --- | --- |
| 平台管理端 | http://localhost:5173 |
| 租户代理端 | http://localhost:5174 |
| API（管理端、代理端及公开下载） | http://localhost:8888 |
| MinIO API | http://localhost:9000 |
| MinIO Console | http://localhost:9001 |
| MySQL | localhost:3306 |
| Redis | localhost:6379 |
| etcd | localhost:2379 |

默认平台管理端账号：

```text
用户名：appforge
密码：AppForge@123
```

默认租户代理端账号：

```text
用户名：agent
密码：AppForge@123
```

默认 MinIO 账号：

```text
用户名：appforge
密码：appforge_dev_minio
Bucket：appforge
```

以上凭据只允许用于本地开发，生产环境不得复用。

## 常用命令

这些命令只管理开发部署，不替代项目各模块的代码生成命令：

```bash
# 启动或更新完整环境；修改 Go、UI 或 Dockerfile 后使用
make up

# 只启动 MySQL、Redis、etcd 和 MinIO；在宿主机运行 Go/UI 时使用
make infra

# 查看状态和日志
make ps
make logs

# 只校验 Compose 配置
make config

# 停止容器但保留数据库和对象存储数据
make down

# 删除全部开发数据卷并重新初始化数据库
make reset-db
```

`make reset-db` 会删除本 Compose 项目的 MySQL、Redis、etcd 和 MinIO 数据卷，仅可用于本地开发。

## 初始化规则

MySQL 仅在 `mysql-data` 为空时执行前三项初始化脚本；随后 `mysql-migrate` 每次启动都会幂等执行迁移：

1. `services/system/system.sql`
2. `services/core/core.sql`
3. `deploy/mysql/init/30-seed.sql`
4. `deploy/mysql/migrations/*.sql`

种子数据只包含：默认租户、平台管理员、代理端租户 Owner、基础角色、两端菜单、API 路由权限和系统/对象存储配置。业务表不写演示数据。平台管理员属于平台作用域和租户 `0`；代理端 Owner 属于默认租户 `1`，两者不能互相调用对方的受保护接口。

菜单与接口权限仍采用 RBAC：

- `owner`：全部菜单和接口权限
- `admin`：应用、版本、渠道、签名配置、构建任务的管理权限
- `viewer`：业务菜单及 GET 查询权限

## APK 上传、构建与下载

- 两个前端容器使用同源 API：Nginx 将 `/admin/`、`/agent/`、`/api/` 和 `/d/` 转发到 `admin-api`，其余路径回退到 Vue 入口以支持页面刷新。
- APK 与 Keystore 由浏览器通过 API 获取预签名地址后直传 MinIO，桶保持私有。
- 版本和签名配置保存对象 ID，不需要客户手工填写 MinIO 地址。
- `builder-worker` 自动领取任务，注入渠道文件、执行 `zipalign`、重新签名、校验并上传 APK 与构建日志。
- 浏览器直传会显示上传百分比；超过时限仍未完成或校验失败的对象由 Worker 定期从私有桶删除并标记为 `DELETED`。
- 代理端可下载授权对象；公开推广链接使用 `http://localhost:8888/d/{channelCode}`，API 记录幂等点击/下载后跳转到短时签名地址。
- `builder_id + builder_attempt + lease` 共同构成任务写入凭据，过期 Worker 无法覆盖被重新领取的任务。
- RPC 服务要求 `InternalRpc.Token`，无内部凭证的直连请求会返回 `Unauthenticated`。生产环境必须使用 Secret Manager 注入独立随机值，不能复用仓库中的开发值。

修改 `system.sql` 或 `core.sql` 后，先按项目根目录 `AGENTS.md` 的规则执行对应服务的 `make gen-model`。已有数据库卷不会自动重放 SQL；确认不需要保留本地数据后，再执行 `make reset-db`。

## 配置与端口

服务启动时从 etcd 读取 `deploy/etcd/*.yaml`。修改这些文件后，执行以下命令重写 etcd 配置并重启应用服务：

```bash
docker compose -f docker-compose.dev.yml up -d --force-recreate etcd-init
docker compose -f docker-compose.dev.yml restart system-rpc core-rpc builder-rpc admin-api
```

如果默认端口冲突，可复制环境变量模板后修改：

```bash
cp .env.example .env
```

宿主机端口变化不会改变容器之间的服务地址。
