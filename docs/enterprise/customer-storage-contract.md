# CUSTOMER_STORAGE 实施契约

## 1. 适用范围

`CUSTOMER_STORAGE` 用于 Hybrid/Private 场景：源 APK、Keystore、品牌资源、构建 APK 和日志的字节始终位于客户对象存储，控制面只保存受约束的对象引用、大小、SHA-256、所有权和状态。Local Agent 继续只主动出站，不新增入站端口、第二套代理前端、任意命令接口或客户对象存储长期凭据托管。

普通 `CONTROL_PLANE_STORAGE` 上传仍由现有 `appforge-ui + appforge-api` 完成。客户存储模式不允许浏览器把本地文件先传给控制面再“转存”；客户在构建节点使用受限 Local Agent 导入命令，控制面页面只负责生成脱敏命令模板、展示返回的对象 ID，并用现有版本与签名配置接口绑定该对象 ID。

## 2. 数据模型

实施时必须显式区分物理存储归属，不得根据对象 Key 前缀猜测：

- `t_storage_object.storage_mode`：`1 CONTROL_PLANE_STORAGE`、`2 CUSTOMER_STORAGE`，历史记录默认 `1`。
- `t_storage_object.owner_agent_id`：客户对象必须记录完成字节复验并登记它的 Agent；控制面对象为 `0`。
- `t_storage_object.object_key`：仅保存桶内相对 Key，不保存 endpoint、bucket、Access Key、Session Token、签名 URL 或查询参数。
- `t_hybrid_artifact_reference`：继续记录 `tenant_id + agent_id + task_id + builder_attempt + artifact_type`、规范引用、大小和 SHA-256。
- 客户侧输入登记后为 `READY`，被版本、签名或品牌配置使用后转为 `BOUND`；构建输出在完成事务中直接登记并绑定为 `BOUND`，同时回填 `t_build_task.apk_object_id/log_object_id`。

客户对象仍计入租户逻辑存储用量，但控制面清理 Worker 不得向控制面对象存储删除 `storage_mode=2` 的字节。客户侧删除需要单独的 Agent 清理任务和当前对象所有权校验，不纳入本阶段隐式实现。

## 3. 注册引用与本地 Secret

Agent 注册时的 `customer_storage_ref` 使用规范格式：

```text
local-file:///customer-storage.json#tenants/<tenantId>/agents/<agentCode>
```

Fragment 是控制面可见、无凭据的已登记桶内前缀；必须规范化、不得包含 `..`、百分号编码、查询串、用户信息或控制字符，并且必须匹配当前 tenant 与 Agent code。URI path 只指向 Agent Secret 根目录内的私有普通 JSON 文件。

本地 Secret 使用严格 JSON，拒绝未知字段和尾随数据：

```json
{
  "provider": "minio",
  "endpoint": "https://objects.customer.example",
  "region": "us-east-1",
  "bucket": "appforge-private",
  "prefix": "tenants/900101/agents/build-a",
  "access_key_id": "...",
  "access_key_secret": "...",
  "session_token": "",
  "ca_file": "customer-object-ca.crt"
}
```

Provider 固定为 `s3`、`minio` 或 `aliyun-oss`。文件必须位于配置的 Secret root、不得为符号链接、权限不得向 group/others 开放、最大 64 KiB；`prefix` 必须与注册 URI fragment 完全一致。凭据只进入 Agent 内存和对象存储 SDK，不进入 Task Bundle、`task.json`、进程参数、数据库、日志或诊断包。生产环境优先使用短期 Session Token/工作负载身份和只允许登记前缀的最小权限策略。

## 4. 规范对象引用

控制面和 Agent 只接受以下规范引用，不接受任意 URL：

```text
customer-object://<agentId>/<registeredPrefix>/inputs/apps/<appId>/<objectType>/<sha256>.<ext>
customer-object://<agentId>/<registeredPrefix>/tasks/<taskId>/attempts/<builderAttempt>/built.apk
customer-object://<agentId>/<registeredPrefix>/tasks/<taskId>/attempts/<builderAttempt>/build.log
```

引用不得含凭据、query、fragment、百分号编码、空路径段或路径穿越。Core 必须从已认证 Agent 记录取得 tenant、Agent code、允许应用和注册前缀，不能信任请求中的 tenant/Agent 字段。

输入 Key 以内容 SHA-256 确定，同一应用、类型和内容可幂等重试；同一引用使用不同大小、SHA、类型或应用时返回冲突。输出 Key 由 task、attempt 和类型唯一确定，旧 attempt 永远不能覆盖或完成新 attempt。

## 5. 客户侧输入导入

Local Agent 提供固定 `customer-storage-import` 命令，不接受脚本或自定义远程命令：

1. 从 Agent 私有状态读取 mTLS 身份和注册的 `customer_storage_ref`。
2. 仅接受本机普通文件、应用 ID 和白名单对象类型；Keystore 必须为私有文件。
3. 本地流式计算大小和 SHA-256，生成规范输入 Key。
4. 使用本地 Provider 上传到注册 bucket/prefix；关闭上传流后重新 `Stat + Open` 全部字节并独立计算大小和 SHA-256。
5. 通过现有专用 mTLS Gateway 的固定登记端点提交对象引用和复验结果。
6. Core 重新认证证书、Agent 状态、存储模式、tenant/app 授权、对象类型和注册前缀，幂等创建 `storage_mode=2` 的 `t_storage_object`，返回对象 ID。
7. 客户把对象 ID 填入现有版本、签名、品牌或模板配置；控制面页面不得尝试为客户对象生成控制面 Presigned URL。

建议页面展示的命令模板只包含 Agent code、应用 ID、对象类型和本地占位路径，不包含 Secret、Access Key、对象存储 endpoint 或一次性认证材料。

## 6. 构建数据流

1. Core 领取任务时同时返回 Artifact 模式和无凭据的 `customer_storage_ref`。
2. Gateway 读取 `BuildExecutionContext`，确认每个输入对象为 `storage_mode=2`、同 tenant/app、状态为 `READY/BOUND`、`owner_agent_id` 与当前 Agent 一致或满足明确迁移策略，并生成规范 `customer-object://` 输入引用。
3. Agent 解析本地 Secret，在登记前缀内下载每个输入到权限为 `0700/0600` 的临时目录；下载后独立校验声明大小和 SHA-256，固定执行器只接收本地路径。
4. 固定执行器完成 APK 构建。Agent 对私有普通输出文件计算摘要，上传到当前 task/attempt 的确定 Key，随后重新打开客户对象并完整重算字节数和 SHA-256。
5. Agent 通过 mTLS 回报规范引用和复验元数据。Gateway/Core 校验证书、模式、tenant/app、task 持有者、attempt、Artifact 类型、确定 Key 和幂等冲突。
6. 同一数据库事务内登记并绑定客户输出对象、写入 Hybrid 引用、更新任务对象 ID 和成功/失败状态；任一步失败都不能留下“任务成功但对象未绑定”的状态。

客户存储票据不复用控制面 Redis GET/PUT Ticket；长期凭据只在 Agent 本地。mTLS 只保护 Agent 与控制面的元数据/任务通信，对象字节直接在 Agent 与客户存储之间传输。

## 7. 下载、清理与可观测性

- 控制面下载接口遇到 `storage_mode=2` 必须返回结构化的“客户侧访问”结果或拒绝生成控制面 URL，不能把相对 Key 错发到平台私有桶。
- 日志和审计只记录 provider 类型、Agent ID、task/attempt、对象类型、大小、SHA 和结构化结果；不得记录 endpoint 完整路径、bucket、Secret 引用内容、凭据或签名 URL。
- 诊断包继续排除 Agent Secret、Keystore、对象内容和任务日志正文。
- 凭据轮换通过覆盖本地私有 Secret 文件并重启/热加载 Provider 完成；控制面无需接收新凭据。

## 8. 安全失败与恢复

- 错误/吊销/过期证书、错误 Agent、跨 tenant/app、错误对象类型、越界 prefix、非规范引用全部拒绝。
- 上传后对象被替换、截断或摘要变化时，客户侧重新读取校验失败，不得登记或完成任务。
- 同一幂等键使用不同引用、大小或 SHA 时冲突；完全相同的重试返回原结果。
- Agent 中断后租约回收推进 attempt；旧进程即使完成上传也不能回写新任务，旧对象保留为未绑定候选并由显式清理流程处理。
- Drain 停止领取新任务但允许当前 attempt 续租和完成；证书吊销立即阻止所有后续登记、续租和完成请求。

## 9. 验收矩阵

实现完成至少需要以下证据：

| 类别 | 必须证明 |
| --- | --- |
| 正向 E2E | 临时 MinIO、合成 APK/Keystore、真实固定执行器、客户输入导入、构建、输出/日志上传、重新读取、签名与包名校验全部成功 |
| 身份与范围 | 错误证书、错误 Agent、跨租户、未授权应用和越界前缀均失败 |
| 完整性 | 输入篡改、上传后输出篡改、错误大小/SHA 和对象替换均失败 |
| 幂等与 fencing | 相同登记可重试；不同元数据冲突；旧 attempt 不能完成；新 attempt 可恢复 |
| Secret 泄漏 | Task Bundle、数据库、容器日志、构建日志和诊断包扫描不到 Access Key、Session Token、Keystore 密码和签名 URL |
| 数据归属 | 控制面私有对象存储中不存在客户模式 APK/Keystore/构建 APK/日志字节 |
| 清理 | 验收结束删除临时租户、Agent、证书、MinIO bucket/volume 和合成文件，不读取或修改现有客户数据 |

真实 S3、阿里云 OSS 和客户工作负载身份需要各自环境验收；临时 MinIO 只证明 S3 兼容基础链路，不能替代所有 Provider 的正式证据。

## 10. 实施授权边界

本功能的实现与测试仅在明确授权下读取受限 `local-file://` 测试 Secret，并只访问临时 MinIO 的登记前缀、合成 APK、合成 Keystore、构建产物和日志。该授权不覆盖真实客户数据、生产凭据、登记前缀外对象、bucket 创建或 policy 修改；真实 S3/OSS 上线仍需客户环境单独验收和授权。
