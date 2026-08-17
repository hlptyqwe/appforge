# AppForge 版本兼容矩阵

| 控制面 | 数据库 Schema | Local Agent 协议 | 支持升级来源 | 回滚边界 |
| --- | --- | --- | --- | --- |
| 1.0.x | `20260814_110_v4_builder_node_recovery` | 1、2 | 0.9.x、1.0.x | 应用镜像可回滚；108–110 均为扩展迁移，可回滚应用但保留新增表和权限 |
| 1.1.x | `20260815_112_v7_customer_storage` | 2、3 | 1.0.x、1.1.x | 协议3增加Task Bundle；111增加只读部署菜单权限，112增加对象存储模式和Agent归属字段；应用可回滚并保留110–112扩展迁移，回滚后协议3 Agent停止领取新任务且不得调度CUSTOMER_STORAGE任务 |
| 1.2.x | `20260815_113_v7_air_gapped` | 2、3 | 1.1.x、1.2.x | 113增加AIR_GAPPED任务包状态和平台/租户六条细粒度权限；迁移不给已有角色自动授权；应用可回滚但保留113扩展迁移，回滚到1.1.x前必须停止AIR_GAPPED新任务并完成或撤销已锁定包 |

规则：

- 控制面遵循语义化版本；Agent 在当前协议及前一个协议窗口内可心跳和轮换证书。完整 Task Bundle 要求协议3，协议1、2 Agent不得领取依赖该能力的新任务。
- 超出窗口的 Agent 仍可心跳和轮换证书，但不能领取新任务。
- 升级前必须完成配置、容量、备份校验，且目标版本跨度不得超过矩阵。
- 迁移脚本只做 expand；删除/收缩在跨过一个兼容窗口后的独立版本执行。
- 数据库已执行不可逆迁移时，不承诺仅靠旧镜像完成伪回滚；应恢复升级前备份到独立环境。
- 当前 1.2.x 正式容器镜像平台为 `linux/amd64`；离线介质必须在 `PLATFORM` 中明确目标，并用 `PLATFORM-IMAGES` 记录部署标签、目标平台子 manifest digest 与已签名索引 digest 的对应关系。双架构交付 CLI 不代表容器镜像已经支持 ARM64。

当前 1.2.x 协议门禁如下；该表是运行时行为，不是仅供展示的版本说明：

| Agent 协议 | 心跳状态 | 证书轮换 | 领取新任务 |
| --- | --- | --- | --- |
| 1 | `UPGRADE_REQUIRED` | 允许 | 拒绝 |
| 2 | `ONLINE` | 允许 | 拒绝；当前任务使用协议3 Task Bundle |
| 3 | `ONLINE` | 允许 | 允许 |
| 4 及更高 | `UPGRADE_REQUIRED` | 允许 | 拒绝，防止未来协议被旧控制面误调度 |

协议窗口由 `common/agentprotocol` 在控制面、Local Agent 和 AIR_GAPPED 路径共享；部署配置只能声明编译时窗口 `2–3`，漂移会导致启动配置校验失败。真实本地 MySQL 验收已覆盖协议1–4的状态切换、升级所需证书轮换、领取拒绝、协议3正常领取及临时数据清理，证据见 `evidence/v7-agent-protocol-window-20260817.json`。协议实现变更后的公开当前源码 Compose 又通过 3600 秒代表性尺寸 API/APK/临时 MinIO 混合回归、强制阈值/零重启校验、Artifact 上传和环境清理，证据见 `evidence/v7-agent-protocol-window-soak-public-ci-20260817.json`；该合成回归不替代旧版正式 Agent 和客户现场升级验证。

当前连续迁移说明：`108` 增加 V7 企业交付业务表，`109` 增加 Local Agent 菜单权限，`110` 增加 Builder 隔离节点恢复权限，`111` 增加只读企业部署状态菜单权限，`112` 增加客户存储对象模式和 Agent 归属字段，`113` 增加 AIR_GAPPED 状态与六条细粒度权限目录。正式发版前必须在空库和上一兼容版本数据库各执行一次迁移验证。

`113_v7_air_gapped` 已提升为 1.2.x 生产目标。生产 Compose、Helm、迁移镜像、API 部署状态和 Release Workflow 均固定为该目标，并增加 1.2.x/Schema 113 不匹配拒绝门禁。隔离验收通过空库安装、112→113 升级、重复执行、升级前标记保持、六条平台/租户细粒度权限及已有角色零自动授权；证据见 `evidence/v7-schema113-production-20260816.json`。历史候选证据 `evidence/v7-schema113-candidate-20260816.json` 仅保留为提升前记录。

本地 kind v1.32.2 已验证应用镜像 `1.2.0 → 1.2.1` 滚动升级后回滚到 revision 1，回滚期间不回退数据库；累计 350 次集群内 API 健康探测零失败，回滚后 `1.2.0` 双副本继续使用 Schema 113，AIR_GAPPED 表保留。机器证据见 `evidence/v7-kubernetes-upgrade-20260816.json`。本地新旧标签使用同一发布镜像摘要，因此该证据验证 Helm 编排、可用性和 Schema 兼容边界，不证明真实版本二进制差异，也不替代客户目标 Kubernetes、CSI 和 Ingress 的交付复验。

正式 Kubernetes 门禁已在原生 `linux/amd64` GitHub Runner 使用公开正式 tag 完成 `v1.2.5 → v1.2.6 → v1.2.5`：双版本聚合 Sigstore 身份和 21 个镜像实时 Cosign 均通过，9/9 个 Helm 应用镜像 Image ID 不同；升级 205 次、回滚 366 次集群内 API 探测全部成功且失败 0 次，回滚后 Schema 113、AIR_GAPPED 表和两个 API 副本保持。机器证据见 `evidence/v7-formal-kubernetes-upgrade-v1.2.5-v1.2.6-20260817.json`。该证据补齐真实版本二进制差异，但仍不替代客户目标 CSI、Ingress、私有仓库和现场升级记录。

本地 Compose 双 internal 网络先使用预载同摘要开发镜像完成 `1.2.0 → 1.2.1 → 1.2.0` 离线应用升级与回滚，作为快速编排回归。随后使用两个公开正式 tag 的签名介质完成 `1.2.5 → 1.2.6 → 1.2.5`：两个 Release 对应不同 Git commit，16/16 个 AppForge `linux/amd64` 平台镜像摘要不同；目标版本迁移镜像、应用服务和 MySQL/etcd/MinIO 均实际切换后回滚，发布部署文件同步切换，数据库与对象标记保持，Schema 始终为 113。261 次连续探针失败 6 次，最大连续失败 3/门禁 10；证据见 `evidence/v7-formal-offline-upgrade-v1.2.5-v1.2.6-20260817.json`。112→113 Schema 跨版本升级另由生产迁移验收覆盖。该证据证明正式版本差异的本地禁拉取升级/回滚，但不替代客户物理断网、正式介质交接、客户硬件或目标环境验收。
