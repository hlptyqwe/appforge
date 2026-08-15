# Local Agent 客户侧安装与升级

此目录是 Hybrid/Private 模式的客户侧交付包。它只启动一个主动出站的 Local Agent，不发布端口，不包含第二套前端或 API。控制面仍由现有 `appforge-ui` 和 `appforge-api` 提供。

## 首次安装

1. 安装 Docker Engine 24+ 与 Compose v2，将本目录复制到客户构建节点。
2. 复制 `.env.example` 为 `.env`，填写固定版本镜像、HTTPS 控制面地址和 mTLS Gateway 地址；禁止使用 `latest`。
3. 把控制面 HTTPS CA 和 Gateway HTTPS CA 分别保存为 `runtime/control-ca.crt`、`runtime/gateway-ca.crt`。公有 CA 也应提供对应根证书，避免依赖镜像内证书集合变化。
4. 在租户管理页面创建 Local Agent 和一次性注册码，然后运行 `./register.sh`。脚本交互读取注册码并通过标准输入传入，注册码不会写入 `.env`、Compose 文件或进程参数。
5. 运行 `docker compose ps`，确认 `local-agent` 为 `healthy`。Agent 只需要到控制面 HTTPS 与 Gateway 端口的出站网络权限。

注册产生的客户端私钥、证书和防重放时间戳保存在 Docker 卷 `agent-state`。不得复制到日志或工单；备份时应使用客户密钥加密并限制恢复目标。删除该卷会丢失 Agent 身份，必须在控制面吊销旧证书并重新注册。

## 导入本地签名 Secret

准备权限为 `0600` 的临时 JSON 文件：

```json
{"keystorePassword":"...","keyPassword":"..."}
```

执行 `./secret-import.sh app-release.json /secure/path/app-release.json`。导入命令只接受单层 `.json` 文件名和固定字段，内容通过标准输入写入 Docker 管理卷 `agent-secrets`；控制面签名配置只填写 `local-file:///app-release.json`。确认任务预检成功后安全销毁临时明文文件。

## 升级与回滚

1. 在控制面把 Agent 设为 Drain，等待运行任务归零。
2. 对照控制面发布包中的 `docs/version-compatibility.md` 确认协议窗口。
3. 在线环境执行 `./upgrade.sh --drained registry.example.com/appforge/local-agent:1.1.x`。物理断网环境先从已验签的离线交付包导入镜像，再执行 `./upgrade.sh --drained --offline registry.example.com/appforge/local-agent:1.1.x`；离线模式只接受本机已经存在的固定版本镜像。
4. 脚本检查协议3、保留状态与 Secret 卷、等待健康检查；新容器不健康时自动恢复原镜像，并再次确认原镜像健康后才返回。
5. 验证心跳、证书到期时间和能力后，在控制面解除 Drain。

## 安全边界

- 容器固定以 UID/GID `65532`、只读根文件系统、无 Linux capabilities 和 `no-new-privileges` 运行。
- 不开放入站端口；健康检查只读取本地私有状态、证书有效期和 mTLS 密钥对，不主动输出私钥或 Secret。
- 构建任务只能调用镜像内固定 `appforge-local-build`，控制面不能下发 shell、脚本或任意命令。
- `agent-state` 和 `agent-secrets` 是敏感卷。卸载或迁移前必须先 Drain/吊销，并按客户密钥管理规范备份或销毁。
