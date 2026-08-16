# AIR_GAPPED Artifact 实施契约

## 1. 适用范围

`AIR_GAPPED` 用于控制面与客户构建节点之间没有网络连通的 Hybrid 场景。控制面把当前构建 attempt、输入 Artifact 和控制面签名封装为离线任务包；客户通过受控介质把任务包送到已注册 Local Agent；Agent 离线验证并运行固定 APK 构建器，再输出由 Agent 当前客户端证书私钥签名的结果包；控制面导入后重新验证字节、证书、签名、防重放状态和当前 fencing attempt。

本模式不新增代理前端、远程命令、脚本字段或离线通用执行器。任务导出和结果导入复用现有 `appforge-api` 管理端、Core 任务状态和控制面私有对象存储。离线介质搬运由客户负责，AppForge 不自动挂载 U 盘、共享目录或客户文件系统。

## 2. 状态模型

新增 `t_air_gapped_package`，一行绑定一个 `tenant_id + task_id + builder_attempt`：

- `package_code`：不可猜测的公开包 ID，不包含 tenant、Agent 或凭据。
- `agent_id`、`task_id`、`builder_attempt`：固定离线执行身份和 fencing 键。
- `nonce_hash`：只保存导出 nonce 的 SHA-256；明文 nonce 只存在受签名任务清单中。
- `export_object_id/sha256/size_bytes`：控制面生成并重新读取确认的任务包。
- `result_object_id/sha256/size_bytes`：用户上传且控制面完成导入的结果包。
- `status`：`1 PREPARING`、`2 EXPORTED`、`3 IMPORTED`、`4 EXPIRED`、`5 REVOKED`。
- `expires_at/imported_at`：导入时同时检查包有效期和任务租约。

任务包和结果包物理文件使用控制面私有对象存储，`t_storage_object.storage_mode=1`、`owner_agent_id=0`；传输模式记录在 `t_hybrid_artifact_reference.storage_mode=3`。任务包、结果包和输出 APK/日志使用不同对象类型，不根据文件名猜测用途。

## 3. 任务导出

租户管理员指定 `taskId + agentId + expiresSeconds`：

1. Core 锁定 Agent 和任务，要求 Agent 为 `AIR_GAPPED`、未吊销、协议3、接受任务、pool/app/capability 匹配且当前证书有效。
2. 任务必须为 `PENDING`，或为租约已过期且可恢复的运行状态。Core 递增 `builder_attempt`、绑定 `local-<agentId>`、创建槽位租约和离线包状态；导出本身等价于领取任务。
3. API 获取不可变 `BuildExecutionContext`，要求所有输入为控制面对象、同 tenant/app、`READY/BOUND`，并从私有对象存储完整读取每个输入，重新核对大小和 SHA-256。
4. API 生成严格版本化任务清单；Core 使用 Agent CA 的 ECDSA 私钥签署清单 SHA-256。签名只覆盖规范 JSON 字节，不签任意 ZIP 元数据。
5. API 生成确定性 ZIP，上传到控制面私有对象存储并重新读取整个包计算大小和 SHA-256；Core 确认导出对象和离线状态后才返回短时下载地址。

任务清单固定包含：schema、package code、nonce、tenant、Agent、Agent 证书序列号、task/attempt、签发/过期时间、完整 Task Bundle、每个输入在 ZIP 中的相对路径/大小/SHA、控制面签名算法和签名。不得包含控制面对象 Key、Presigned URL、Access Key、数据库凭据、主密钥、签名密码密文或任意命令。

## 4. 离线 Agent 构建

Local Agent 提供固定 `air-gapped-build` 命令：

1. 任务包、结果包、状态目录和 Secret root 必须是本地绝对路径；输入不得为符号链接，输出必须尚不存在。
2. 严格解析 ZIP：拒绝重复 entry、绝对路径、`..`、符号链接、未知文件、压缩炸弹、超过声明大小、尾随清单字段和非规范 JSON。
3. 使用本地固定 Agent CA 证书验证控制面 ECDSA 签名；校验当前时间、tenant/Agent、任务、attempt、证书序列号、nonce 和包 ID。
4. 对每个输入完整解压并独立重算大小/SHA；任务包 SHA 与控制面声明不一致时拒绝。
5. 使用本地 `local-file://` 签名 Secret 和固定 `appforge-local-build` 执行，不向执行器传递控制面签名、Agent 私钥、nonce 或离线包路径。
6. 对 APK/日志完整计算大小/SHA，生成严格结果清单；使用 Agent 当前 ECDSA 客户端私钥签署结果清单 SHA-256，并附带客户端证书。
7. 原子写入结果 ZIP 后，在 Agent 私有状态目录记录已消费 `package_code + nonce_hash`。同一任务包不得再次执行，即使结果文件被删除。

任务包验证完成前不得解析 Keystore 或启动构建器。构建失败也输出受签名失败结果和可选日志；结果清单不得包含签名密码、Secret 内容或执行器环境。

## 5. 结果导入

用户先通过现有私有上传链路上传结果 ZIP，再调用固定导入接口提交结果对象 ID：

1. API 重新读取整个结果对象，严格解析 ZIP 并重算容器、APK 和日志大小/SHA。
2. Core 按 `package_code` 锁定离线状态、Agent、证书和任务；校验 nonce hash、任务包 SHA、tenant/Agent/task/attempt、有效期、当前 task owner/lease 和未消费状态。
3. Core 验证 Agent 证书由当前 Agent CA 签发、SPIFFE tenant/Agent 匹配、证书记录存在且未吊销，并用该证书验证规范结果清单签名。
4. API 把已验证 APK/日志分别写入控制面私有对象存储，重新打开并核对，再登记为独立 `t_storage_object`。
5. Core 在同一事务中绑定 APK/日志对象、写入 mode 3 Hybrid 引用、更新任务成功/失败状态、关闭槽位租约并把离线包置为 `IMPORTED`。
6. 完全相同的成功导入可返回原结果；同一包 ID、nonce 或 attempt 的不同结果必须冲突。已过期或被恢复到新 attempt 的包永远不能回写。

控制面只为成功导入后的 APK 提供现有私有下载/渠道分发能力。结果 ZIP 是审计传输载体，不作为最终 APK 引用。

## 6. 签名与规范化

- 控制面签名：`ECDSA_P256_SHA256`，公钥为 Agent 注册时固定到本地的 Agent CA。
- Agent 签名：`ECDSA_P256_SHA256`，公钥为任务清单指定序列号对应的 Agent 客户端证书。
- 签名编码固定为 ASN.1 DER 的 Base64；拒绝裸 `r/s`、算法协商、未知算法和非 P-256 密钥。
- Manifest 使用版本化 Go struct 编码的紧凑 JSON；导入端重新编码后必须与原始字节完全一致，借此拒绝字段重排以外的模糊表示、未知字段和尾随数据。
- ZIP entry 名称固定使用 `/`，时间戳归零，方法固定为 Store；安全判断不能依赖 ZIP CRC。

## 7. 失败、恢复和清理

- 导出中断：状态保持 `PREPARING`，任务租约到期后可恢复为新 attempt；旧包即使后来完成上传也不能 Finalize。
- 离线构建中断：未生成原子结果包；本地消费标记在结果包成功落盘后写入，可由同一命令在未消费状态下重试。
- 导入中断：未完成 Core 事务时任务不成功；重复导入先核对离线状态和对象摘要。
- 包过期、证书吊销、Agent Drain、任务取消或 attempt 推进后导入失败。
- 控制面清理 Worker 只清理控制面对象；不会访问客户介质或 Agent 本地目录。

## 8. 验收矩阵

| 类别 | 必须证明 |
| --- | --- |
| 正向 E2E | 合成 APK/Keystore → 控制面签名任务 ZIP → 断网 Agent 真实构建 → Agent 签名结果 ZIP → 控制面导入 → APK 签名/包名/摘要正确 |
| 双向信任 | 错误 CA、错误 Agent 证书、错误 agentId/tenant/task/attempt、吊销证书均失败 |
| 完整性 | 任务清单、输入、结果清单、APK、日志、ZIP 容器任一篡改均失败 |
| 防重放 | Agent 本地重复执行失败；控制面重复导入幂等；不同结果冲突；过期包和旧 attempt 失败 |
| 数据泄漏 | ZIP 清单、数据库、日志、诊断包均不含签名密码、Secret、Access Key、对象 Key或短时 URL |
| 断网 | Agent 构建容器使用 `--network none`，成功过程不访问控制面、DNS、对象存储或外部服务 |
| 清理 | 验收结束删除临时租户、包对象、证书、状态卷和合成文件，不读取真实客户数据 |

## 9. 当前授权边界

仓库内实现和本地验收只使用合成 APK、合成 Keystore、临时证书、临时租户和本地 Docker 私有对象存储。不得读取真实客户介质、生产 Keystore、生产证书或现有客户离线包；任何真实客户环境导出、介质搬运或导入都需单独授权。

## 10. 当前实施检查点

截至 2026-08-16，AIR_GAPPED 运行闭环与合成验收已经落盘：

- `t_air_gapped_package` Schema 与 113 幂等迁移；
- 任务包、结果包的独立 Storage Object/Hybrid Artifact 枚举；
- Core 已实现准备导出、任务 attempt/槽位/额度锁定、Manifest 签名、导出确认、撤销、状态查询和结果导入事务；
- API 已实现确定性任务 ZIP、私有对象上传回读、结果 ZIP 严格解析、输出对象二次写入与幂等导入；
- 共享库已实现规范 JSON、Artifact 基数、确定性 Store ZIP、大小/SHA、路径和 ECDSA P-256 低 S 签名校验；
- Local Agent 已实现固定 `air-gapped-build`、控制面 CA 验签、当前 Agent 证书/私钥绑定、私有解压、固定 root-owned 执行器、Agent 结果签名、原子结果发布和持久防重放；
- Schema 113 已登记平台端和租户端各三条导出/导入/查询权限，不自动修改已有角色；租户门户已暴露对应固定路由，Local Agent 管理页按权限提供任务锁定导出、短时下载、状态查询、结果 ZIP 私有上传和签名导入；
- `deploy/acceptance/v7-air-gapped.sh` 已使用临时租户、临时证书、合成 APK/Keystore、本地 MinIO 与 Docker 私有卷，先证明未授权租户角色返回 403、再显式授予专用权限，随后通过任务锁定、额度确认、错误 Agent、输入/结果篡改、`--network none` 真实构建、本地重放、事务导入、重复导入幂等及最终 APK 签名/包名验收，并在清理后验证临时角色授权、租户、对象前缀、容器与私有卷均归零；0600 机器证据见 [v7-air-gapped-20260815.json](./evidence/v7-air-gapped-20260815.json)；
- `deploy/acceptance/v7-schema113-candidate.sh` 已在两个隔离数据库验证 Schema 113 空库安装、112→113 升级、重复迁移、升级前标记保持、六条固定权限目录和已有角色零自动授权；测试还修复了 V1 历史迁移重放吸收后续权限的问题，0600 证据见 [v7-schema113-candidate-20260816.json](./evidence/v7-schema113-candidate-20260816.json)。

本次授权不包含真实客户数据或生产凭据；Schema 113 仍不是生产 Compose、Helm 或 Release Workflow 的默认目标。正式提升该生产边界需要明确发布授权，客户物理断网介质流程和正式证书/镜像仍需目标环境复验；这些交付缺口不否定上述合成运行闭环，但 V7 总状态仍保持“实施中”。
