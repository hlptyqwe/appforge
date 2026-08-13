# AppForge 开发环境部署

本目录提供可直接启动的本地开发环境，包含：

- MySQL 8.4：初始化 `system` 与 `core` 表结构和开发基础数据
- Redis 7.4：模型缓存与权限缓存
- etcd 3.6：初始化 common、system、core、builder、admin-api 配置
- MinIO：APK、keystore 和构建产物的开发对象存储
- system RPC、core RPC、builder RPC、admin API、admin UI

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
| 管理后台 | http://localhost:5173 |
| Admin API | http://localhost:8888 |
| MinIO API | http://localhost:9000 |
| MinIO Console | http://localhost:9001 |
| MySQL | localhost:3306 |
| Redis | localhost:6379 |
| etcd | localhost:2379 |

默认后台账号：

```text
用户名：appforge
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

MySQL 仅在 `mysql-data` 为空时执行初始化脚本，执行顺序是：

1. `services/system/system.sql`
2. `services/core/core.sql`
3. `deploy/mysql/init/30-seed.sql`

种子数据只包含：默认租户、`owner/admin/viewer` 三个角色、一个 owner 账号、前端菜单、API 路由权限和系统/对象存储配置。业务表不写演示数据。

菜单与接口权限仍采用 RBAC：

- `owner`：全部菜单和接口权限
- `admin`：应用、版本、渠道、签名配置、构建任务的管理权限
- `viewer`：业务菜单及 GET 查询权限

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
