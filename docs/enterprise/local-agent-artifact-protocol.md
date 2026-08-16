# Local Agent Task Bundle 与 Artifact 协议

## 1. 目标与边界

Local Agent 继续复用现有 `appforge-api` 的专用 mTLS Gateway、Core 的任务租约和普通 Builder 的 `BuildExecutionContext`，不新增代理前端、代理 API 或第二套构建业务服务。

控制面只允许 Agent 执行固定 APK 构建生命周期，不提供 shell、脚本或任意命令字段。每次请求都必须同时通过客户端证书、Agent 状态、tenant/app 授权、任务持有者和 `builderAttempt` fencing 校验。

## 2. 协议版本

- 协议 1、2：保留注册、心跳、领取、续租、进度、完成/失败和证书轮换基础能力。
- 协议 3：增加完整 Task Bundle、短时 Artifact Ticket 和控制面对象字节级校验。
- 控制面可以接受兼容窗口内的旧 Agent 心跳和证书轮换，但只有支持任务所需协议与能力的 Agent 才能领取新任务。

## 3. Task Bundle

领取成功后，Gateway 根据 `taskId + local-{agentId} + builderAttempt` 读取不可变构建上下文，并返回：

- 任务、租户、应用、渠道、版本和 attempt。
- 包名、API Host、渠道名和落地页。
- 品牌快照、白标模板快照和签名证书指纹。
- 源 APK、Keystore、Logo、启动图和模板文件清单。
- 每个控制面输入对象的稳定引用、声明 SHA/大小和短时只读 URL。
- Key Alias，以及任务执行所需的最小签名 Secret。控制面密文只在 Gateway 内存中解密，不下发主密钥或密文；本地 Secret 引用由 Agent 本地 Provider 解析。
- APK 与构建日志的预留对象引用和短时只写 URL。

Task Bundle 不包含可执行命令、对象存储 Access Key、数据库凭据、内部 RPC Token、控制面主密钥或其他租户数据。

Agent 不把签名密码写入 `task.json`。密码只通过受限子进程环境变量传给固定构建执行器，任务结束后清空引用并删除权限为 `0700/0600` 的临时目录。

## 4. Artifact 模式

### 4.1 CONTROL_PLANE_STORAGE

1. Gateway 为已授权输入生成短时 GET URL，为当前 task/attempt 的 APK 和日志引用生成短时 PUT URL。
2. Agent 下载后必须按 Bundle 中的大小和 SHA-256 校验，再交给构建执行器。
3. Agent 对本地产物计算大小和 SHA-256，通过预留 URL 上传。
4. 完成请求到达 Gateway 后，Gateway 从对象存储重新读取字节、计算 SHA-256，并校验期望的 tenant/task/attempt 对象引用。
5. Core 只接受 Gateway 已核验的元数据，登记 `t_storage_object`、`t_hybrid_artifact_reference` 和构建结果。

短时 URL 只是传输能力，不写入数据库、审计事件或构建日志。URL 过期时 Agent 必须通过固定的 Artifact Ticket 刷新接口获取新票据，不能提交自定义对象 Key。

### 4.2 CUSTOMER_STORAGE

控制面保存客户存储连接的 Secret 引用，不保存长期访问凭据。Agent 通过本地 Provider 获取客户 S3/MinIO/OSS 凭据，输入输出引用必须位于租户和 Agent 注册时绑定的前缀。完成时至少校验对象元数据、大小、SHA、task/attempt 和所有权；可访问部署应由 Gateway 或客户侧校验服务重新读取字节。

客户源 APK/Keystore 不通过控制面浏览器上传链路中转。客户侧导入、显式存储归属字段、规范对象引用、mTLS 元数据登记、输出绑定和负向验收的实施级约束见 [CUSTOMER_STORAGE 实施契约](./customer-storage-contract.md)。该链路已完成临时 MinIO 合成数据 E2E；真实 S3/阿里云 OSS 仍需客户环境复验。

### 4.3 AIR_GAPPED

控制面导出规范化任务清单、输入 Artifact 清单和签名。离线 Agent 验证控制面签名、有效期、tenant、agent、task、attempt 和 nonce 后构建，再输出包含产物清单、SHA 和 Agent 签名的结果包。导入时必须同时验证双方证书链、签名、防重放和当前 fencing attempt。

状态表、规范 ZIP、双向 ECDSA 签名、导出/导入事务、本地防重放和断网验收的实施级约束见 [AIR_GAPPED Artifact 实施契约](./air-gapped-artifact-contract.md)。

## 5. 幂等与恢复

- Artifact 幂等键为 `tenantId + taskId + builderAttempt + artifactType`。
- 同一幂等键只能重复提交完全相同的引用、SHA 和大小。
- Agent 中断后由租约回收推进 attempt；旧 attempt 的 URL即使尚未过期，完成接口也必须拒绝回写。
- Drain 停止领取新任务，进行中的任务继续续租；超时任务按普通 Builder 规则恢复。
- Gateway 只能为当前证书所属 Agent 正在持有的 task/attempt 签发或刷新票据。

## 6. 验收门禁

每种模式至少执行一次真实 APK 构建，并证明：

1. 输入字节被 Agent 独立校验。
2. 输出字节被控制面或受信客户侧校验器独立重算 SHA。
3. 篡改大小、SHA、对象引用、tenant、task 或 attempt 会失败。
4. URL、Secret、Keystore 和私钥不进入日志、数据库明文字段或诊断包。
5. Agent 中断、Drain、证书吊销和 attempt 推进后，旧进程不能完成任务。

`CONTROL_PLANE_STORAGE`、`CUSTOMER_STORAGE` 与 `AIR_GAPPED` 均已使用合成 APK/Keystore 完成真实构建 E2E；客户对象存储、物理断网介质和正式证书环境仍需分别复验，不得用本地合成证据替代客户交付证据。

## 7. 当前落地状态

截至 2026-08-15：

- 协议3版本门禁、白名单 Task Manifest 和客户侧 `local-file://` 签名 Secret Provider 已实现并通过测试。
- Manifest 不序列化控制面密码密文；需要控制面密文的签名配置或白标敏感参数会返回结构化阻断原因。
- 仓库内固定执行器 `appforge-local-build` 已实现；任务 JSON 不允许命令字段，并对输入路径、权限、大小和 SHA-256 做强校验。Docker 真实验收已完成最小 APK 的渠道注入、对齐、证书指纹核验、签名、包名/内置快照核验和产物摘要输出。
- Core 已对 `CONTROL_PLANE_STORAGE` 强制使用规范的 `storage-object://ID` 引用，并在同一任务事务内校验对象 tenant、app、Artifact/存储对象类型、READY/BOUND 状态、大小和 SHA-256；跨租户、跨应用、错误类型、未完成上传和完整性不一致均拒绝。成功对象会绑定为 BOUND，APK/日志对象 ID 会写回构建任务，防止仅登记任意 URL 或声明元数据。
- `CONTROL_PLANE_STORAGE` 已在获得明确授权后启用：Gateway 使用 Redis TTL 保存票据哈希及 task/attempt 状态，签发绑定 mTLS 证书、原子单次消费、5 分钟有效且仅含相对路径的 GET/PUT 票据；Agent 不接受绝对主机、查询串或越界路径。
- Agent 将下载输入保存为 `0700/0600` 临时文件并独立校验大小/SHA；固定执行器只接收本地路径，不接收票据、控制面 Secret 引用或对象 Key，APK/日志输出在上传前强制收口为私有普通文件。
- Gateway 不接受 Agent 指定对象 Key；上传流由控制面生成租户对象 Key 并首次计算 SHA，完成前再从私有对象存储读取全部字节独立重算大小/SHA，随后 Core 完成元数据、BOUND 绑定和任务对象 ID 回填。
- `deploy/acceptance/v7-control-plane-storage.sh` 已使用临时真实 APK、Keystore 和证书通过完整 E2E，并覆盖错误证书消费、票据重放、上传后对象篡改、旧 attempt fencing 和 attempt 2 恢复。验收结束自动删除临时租户、对象、证书卷和容器，不读取现有客户数据。
- `CUSTOMER_STORAGE` 已实现同等级真实数据搬运与独立字节校验，并通过受限临时 MinIO E2E；`AIR_GAPPED` 已实现双向签名 ZIP、任务/额度锁定、本地防重放、`--network none` 构建和事务导入，并通过合成 E2E。
