# 企业 Secret Provider

统一 Secret 引用格式为 `provider://scope/name#version`，业务对象只保存引用，不保存明文。

- `kubernetes://namespace/secret#key`：由 Pod 挂载只读文件，运行时读取。
- `vault://mount/path#field`：使用短期 Kubernetes/AppRole 身份，按任务读取并立即丢弃。
- `aws-sm://region/secret-id#version-stage`：使用工作负载身份调用 Secrets Manager；KMS 只负责解封装数据密钥。
- `remote-sign://profile/key-id#version`：只提交待签名摘要，HSM 私钥永不离开设备。
- `local://name#version`：Local Agent 从操作系统 Keychain、TPM 或权限为 0600 的本地 Secret Store 读取。

Builder 只在单次任务 attempt 的执行窗口解析 Secret；日志只输出 provider、不可逆引用摘要和版本，不输出路径后的敏感值。失败重试重新鉴权，不缓存跨任务明文。

验收应分别使用真实 Vault 开发实例、云工作负载身份的 Secrets Manager/KMS 测试账户和 Agent 本地 Store；模拟服务只能用于单元测试，不能替代交付验收记录。
