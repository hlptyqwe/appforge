# GitHub / GitLab 受控集成

V5 使用现有 `appforge-api` 完成 OAuth 回调，不新增独立 Web/API 项目。浏览器只接收授权 URL 和最终跳转结果；访问令牌通过内部 RPC 立即交给 Core，并以 `secretbox` 密文保存。

## 部署配置

在 `SourceOAuth.GitHub` 和 `SourceOAuth.GitLab` 中配置供应商的 Client ID、Client Secret、授权地址、令牌地址、API 地址和固定回调地址。Client Secret 必须由 Secret Manager 注入，不能提交到仓库。

未配置 Client ID/Secret 时，连接接口返回前置条件错误，不会退化为手工提交访问令牌。

供应商控制台中的回调地址必须与配置的 `RedirectURL` 完全一致：

```text
https://appforge.example.com/public/v1/source-oauth/callback
```

## 安全边界

- GitHub 使用 `repo read:user`，GitLab 使用 `read_api`；平台仍要求用户在开发者中心显式选择仓库形成二次 allowlist。
- HTTP API 永不返回访问令牌、刷新令牌或密文。
- 断开集成会覆盖已保存令牌并立即把集成置为不可用。
- Artifact 只能通过供应商固定的 Release/CI API 和供应商返回的受控下载地址获取，不能提交任意下载 URL。
- CI ZIP 必须且只能包含一个 APK；下载上限 2 GiB，APK 入库前执行结构、大小和 SHA-256 校验。
- 版本、存储对象绑定和 Artifact 来源在 Core 的同一数据库事务中提交。

## 来源记录

每次导入记录平台、授权仓库、Release/CI 类型、供应商 Artifact ID、commit SHA、pipeline/workflow、job、Artifact SHA-256 和私有存储对象 ID。平台不会执行仓库代码。

## 预定义构建触发策略

开发者中心可为已授权仓库创建源码构建触发策略。策略固定以下边界，供应商请求不能覆盖：

- 目标应用、渠道、签名配置、品牌配置、白标产品和构建池。
- 只接受 Release 发布或 CI 成功事件之一。
- Tag/分支 glob 和 Release 附件名或 CI Job/Artifact 名称精确选择器。

创建或轮换策略时只显示一次 Payload URL 和签名 Secret。GitHub 配置 Payload URL、Secret，并选择 `Release` 或 `Workflow runs` 事件；GitLab 配置 URL、Secret token，并选择 `Release events` 或 `Pipeline events`。

公开入口为 `/public/v1/source-webhooks/{github|gitlab}/{token}`。路径 token 在数据库中只保存 SHA-256 摘要；GitHub 使用 `X-Hub-Signature-256` HMAC-SHA256，GitLab 使用 `X-Gitlab-Token`。签名验证、平台、仓库、事件、ref 和 Artifact 选择器全部匹配后才可靠入队。

`source-trigger-worker` 独立容器通过租约处理事件。供应商 delivery ID 保证入队幂等；Artifact 来源保证版本导入幂等；`source_webhook_event_id + channel_id` 保证 Worker 重试不会重复创建渠道构建任务。失败按可重试错误指数退避，达到上限后保留失败审计记录。

生产环境必须把 `SourceOAuth.WebhookBaseURL` 配置为公网 HTTPS 地址。平台不接受调用方提交任意下载 URL，也不会执行仓库代码。
