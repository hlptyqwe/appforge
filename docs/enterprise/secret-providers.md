# 企业 Secret Provider

Builder Worker 已接入可插拔 Secret Resolver。签名配置只保存 Secret 引用，Worker 在单次任务 attempt 的签名阶段读取严格 JSON，使用后清空内存中的密码字段，不跨任务缓存明文。

Secret 内容格式：

```json
{"keystorePassword":"...","keyPassword":"..."}
```

当前实现支持以下引用：

- `local-file:///signing/app.json`：读取 `SecretProviders.LocalRoot` 下的文件。路径必须留在配置根目录内，禁止符号链接，文件不得向 group/others 开放权限。
- `k8s-secret:///signing/app.json`：读取 `SecretProviders.KubernetesRoot` 下的 Kubernetes Secret 投射文件。允许 Kubernetes 原子更新使用的符号链接，但解析后的目标仍必须位于挂载根目录且权限受限。
- `vault://secret/data/appforge/signing`：访问配置的 Vault 地址，支持 KV v1/v2 响应；默认必须使用 HTTPS，通过只读 token 文件和可选 Namespace 鉴权。
- `aws-secretsmanager://prod/appforge/signing?versionStage=AWSCURRENT`：通过 AWS SDK v2 默认凭据链访问 Secrets Manager，也可使用 `versionId`；由 Secrets Manager 完成 KMS 解密。

`MaxSecretBytes` 默认限制为 64 KiB。解析失败时，构建只返回通用错误，不把 Secret 内容写入日志、数据库或控制面事件。

Local Agent 已实现 `local-file://` 本地 Secret Store：客户通过受限 `secret-import` 命令把固定 JSON 从标准输入写入 Docker 私有卷，运行容器只读挂载；解析时继续校验根目录、拒绝符号链接、限制私有权限和大小。当前没有实现 Keychain、TPM、HSM 或远程签名 Provider，不能使用 `remote-sign://`。真实 Vault 临时实例与文件 Provider 已通过运行验收；AWS Provider 已完成单元测试，但仍需客户测试账户和工作负载身份完成真实验收。
