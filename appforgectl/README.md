# appforgectl

`appforgectl` 是 AppForge Open API v1 的无交互命令行客户端，适合开发机和 CI/CD。

## 构建

```bash
make test
make build
```

二进制输出到 `bin/appforgectl`。

## 认证

开发机可写入权限为 `0600` 的配置文件：

```bash
appforgectl auth configure \
  --base-url https://appforge.example.com \
  --api-key "$APPFORGE_API_KEY"
```

CI 中建议直接使用受保护变量，不落盘：

```bash
export APPFORGE_BASE_URL=https://appforge.example.com
export APPFORGE_API_KEY=afk_xxx
```

CLI 不会主动输出 API Key。不要启用 shell 的 `set -x`。

## 标准流程

```bash
appforgectl --json app list
appforgectl --json version upload --app-id 1 --file app.apk --version-code 100 --version-name 1.0.0
appforgectl --json build create --app-id 1 --version-id 10 --channel-id 2 --signing-config-id 3
appforgectl --json --timeout 30m build wait --id 20
appforgectl --json artifact download --id 30 --output channel.apk
```

全局参数必须写在命令之前：`--json`、`--timeout`、`--retries`、`--config`。

退出码：`0` 成功、`2` 参数错误、`3` 认证错误、`5` API/网络错误、`6` 超时。

