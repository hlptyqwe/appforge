# 远程 APK 签名与 HSM 接入契约

## 1. 边界

`REMOTE_APK_SIGNER` 用于客户 HSM、PKCS#11 网关或受控远程签名服务。AppForge 不读取、导出或托管 HSM 私钥；Builder 只把已经完成渠道注入和 `zipalign` 的未签名 APK 发送给预配置的客户签名服务，并接收已签名 APK。

该模式不是 Secret Provider 的别名。Secret Provider 只提供短期连接配置和 mTLS 客户端身份，签名私钥必须始终留在远程签名服务或其 HSM 内。

## 2. Secret

签名配置只保存受限 `secret_ref`。Provider 返回严格 JSON：

```json
{
  "endpoint": "https://signer.customer.internal:9443",
  "keyId": "android-release-2026",
  "caCertificatePem": "-----BEGIN CERTIFICATE-----...",
  "clientCertificatePem": "-----BEGIN CERTIFICATE-----...",
  "clientPrivateKeyPem": "-----BEGIN PRIVATE KEY-----...",
  "serverName": "signer.customer.internal"
}
```

约束：

- `endpoint` 必须是 HTTPS origin，不允许用户名、查询、片段或自定义路径。
- 必须提供独立 CA、客户端证书和私钥；最低 TLS 1.2，禁止跳过证书验证、重定向以及继承环境 HTTP(S) 代理，避免 APK 被非预期代理接收。
- `keyId` 只标识远端非导出密钥，不得包含 PIN、Token 或密码。
- Secret 原文、私钥、APK 正文和远端错误正文不得写入数据库、构建日志或诊断包。

## 3. 固定协议

### 3.1 密钥信息

`GET /v1/info` 返回严格 JSON：

```json
{"schemaVersion":1,"keyId":"android-release-2026","certificateSha256":"64位小写十六进制"}
```

创建配置和执行构建时都必须确认 `keyId` 与证书指纹。证书变化必须创建新签名配置，不能静默替换，以保持 Android 覆盖安装证书连续性。

### 3.2 签名

`POST /v1/sign-apk` 的 Body 是未签名 APK 原始字节，并包含固定请求头：

- `X-AppForge-Schema-Version: 1`
- `X-AppForge-Task-Id`
- `X-AppForge-Builder-Attempt`
- `X-AppForge-Key-Id`
- `X-AppForge-Request-Nonce`：至少 192 bit 随机值，Base64URL 无填充。
- `X-AppForge-Request-Timestamp`：UTC RFC3339Nano。
- `X-AppForge-Unsigned-Sha256`

成功响应 Body 是签名 APK，并原样回显 task、attempt、keyId、nonce、timestamp 和输入 SHA，同时返回 `X-AppForge-Signed-Sha256` 与 `X-AppForge-Certificate-Sha256`。

远端必须在允许时钟偏差内验证时间，并按 mTLS 身份、task、attempt、nonce 和输入 SHA 持久防重放。重复请求不得再次调用 HSM；实现可返回已缓存的相同结果或明确冲突。

## 4. Builder 强制校验

Builder 在上传结果前必须完成：

1. Secret 严格解析、mTLS 握手、固定 endpoint 和禁止重定向。
2. 请求体大小与输入 SHA 校验。
3. 响应 task、attempt、nonce、timestamp、keyId 和输入 SHA 逐项匹配。
4. 响应大小上限与流式 SHA 校验，临时文件使用 `0600` 并原子落盘。
5. `apksigner verify --print-certs` 成功，实际签名证书 SHA-256 等于任务快照。
6. 包名、渠道、品牌和模板快照校验继续通过。

任何网络超时、TLS 错误、重定向、响应篡改、错误证书、重放冲突或字段不匹配都使当前 task attempt 失败；旧 attempt 仍由 V4 fencing 阻止回写。

## 5. 验收边界

仓库内只允许使用合成 APK、测试 Keystore、临时 CA/客户端证书和本地模拟远程签名服务。合成 E2E 必须覆盖正确签名、错误客户端证书、错误 keyId、请求/响应篡改、nonce 重放、超时、证书指纹不匹配和最终 APK 校验。

本地模拟服务只能证明协议客户端和签名边界，不能宣称真实 HSM 已通过。正式交付还必须在客户测试 HSM 上验证不可导出密钥、PIN/会话治理、HA、审计、限流、性能和厂商 PKCS#11/SDK 行为。

当前仓库已启用持久化签名模式及 Core/API/Builder 任务分支。管理 API 使用显式 `REMOTE_APK_SIGNER` 枚举；为兼容 1.1.x/Schema 112 升级来源及当前 Schema 113，持久化规范表示为 `keystore_object_id=0`、空 Keystore Key、非空受限 `secret_ref`，且不保存 Keystore 密码。创建配置时 Builder RPC 会解析 Secret 并通过 mTLS 校验远端 `keyId` 和证书指纹；执行时 Worker 再次固定身份快照并校验最终 APK。调度器只允许声明 `remoteSigning:true` 的节点领取这类任务，Worker 同时要求至少配置一个受支持的 Secret Provider。

协议级防篡改、防重放和超时证据见 `evidence/v7-remote-apk-signing-20260815.json`；公共管理 API → 持久化 → Core 调度 → 真实 Worker → 最终 APK 的隔离任务证据见 `evidence/v7-remote-signing-task-20260815.json`。两份证据均只使用本地模拟服务，不代表真实 HSM 已通过。
