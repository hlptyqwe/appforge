# AppForge 企业交付手册

## 交付模式

- Dedicated：单客户独立控制面、数据库、对象存储和 Builder 池，由平台运维。
- Private：全部组件位于客户环境，可使用 Compose 或 Helm，可断网安装。
- Hybrid：控制面可托管，APK、Keystore、客户对象存储和构建过程留在客户网络，Local Agent 仅主动出站。

## 安装

Compose：复制 `deploy/production/.env.example` 为 `.env`，生成随机 Secret、TLS 证书和 P-256 ECDSA Agent CA，将 `.env`、TLS 私钥、Agent CA 私钥和 SIEM Token 权限设为仅所有者可读写，再运行 `preflight.sh`；门禁会拒绝占位凭据、非 HTTPS、符号链接、证书/私钥不匹配、非 ECDSA Agent CA、即将过期的证书以及向 group/others 开放的私有文件。通过后再启动 `docker compose up -d`。生产镜像必须固定语义化标签和发布清单中的 digest。

Kubernetes：预先创建 MySQL、Redis、对象存储、内部 RPC、JWT、主密钥和 Agent CA Secret；已有 RWX 许可证状态 Claim 的根目录必须预置为 UID/GID `65532` 可写且不得向其他主体开放；填写 Helm values 后执行 `helm lint` 和 `helm upgrade --install --atomic`。Schema 会拒绝空镜像仓库、非 HTTPS 入口、单副本控制面和无版本镜像。本地 kind v1.32.2/Helm 3.17.3 已验证滚动升级与应用回滚期间 API 连续可用、110 expand Schema 可被旧应用继续使用，客户交付仍需在目标 CSI 与 Ingress 上复验。

离线环境使用 `offline-bundle.sh` 在联网交付机导出 OCI 镜像包，在断网环境用 `offline-install.sh` 校验 SHA 后导入。当前脚本已覆盖镜像、控制面部署配置、Local Agent 客户安装包、`licensectl`/`appforgectl` 和企业文档的导出、校验与导入；正式发布流水线按镜像 digest 归档 SBOM、Trivy SARIF、Cosign 签名回验证据及带 Sigstore 证明的 CLI 校验和。只有真实 tag 流水线成功后，才能把这些证据与离线包一起交付。

Hybrid/Private 客户构建节点使用 `deploy/local-agent` 交付包。该包提供非 root、只读根文件系统、无入站端口的 Compose，注册码通过标准输入传递，Agent 身份与签名 Secret 分别保存在 Docker 私有卷，并提供健康检查、Drain 后在线/已导入镜像离线升级和失败自动恢复旧镜像。详细步骤见 [Local Agent 客户侧安装与升级](../../deploy/local-agent/README.md)。

## 升级与回滚

1. 阅读 `version-compatibility.md`，确认版本跨度和 Agent 协议窗口。
2. 执行生产配置 preflight，检查磁盘至少为当前数据库与对象数据的两倍。
3. 执行 `backup.sh`，在独立环境验证校验和与恢复。
4. 先运行 expand 迁移，再滚动升级无状态 API/RPC，最后升级 Worker 与 Agent。
5. 验证 `/healthz`、`/readyz`、迁移版本、构建领取和审计导出。
6. 应用故障且 Schema 兼容时回滚镜像；不可逆迁移必须恢复升级前备份到独立环境。

## 备份恢复与灾备

参考目标为 RPO 15 分钟、RTO 120 分钟；企业可在 values 或 `.env` 中收紧。MySQL 应启用 binlog/PITR，对象存储启用版本化与跨区域复制，etcd 定时快照。`backup.sh` 生成 MySQL 一致性导出、etcd 快照、对象数据和 SHA 清单；`restore.sh` 需要显式确认目标目录并停止写服务。

恢复后对比：租户数、任务状态摘要、对象 ID/Key/SHA、操作审计摘要和最高迁移版本。恢复演练不得覆盖生产环境，应使用独立网络、数据库和对象桶。`deploy/acceptance/v7-backup-restore.sh` 已验证 MySQL、etcd 和对象卷的基础破坏后恢复机制，并对恢复点间隔与破坏到恢复核验完成进行计时；2026-08-15 隔离合成数据结果为 RPO 2 秒、RTO 9 秒，见 `evidence/v7-synthetic-dr-20260815.json`。正式交付仍必须使用完整业务备份执行上述业务摘要核对和客户环境计时。

## 安全与运维

- 所有容器以非 root、只读根文件系统、drop capabilities 和 no-new-privileges 运行。
- 生产 Secret 只由 Kubernetes Secret、Vault/KMS 或外部 Secret Operator 注入，不写入 values、Compose 文件、日志和诊断包。
- NetworkPolicy 默认拒绝，仅放行组件间、DNS、入口 Namespace 和明确外部依赖出站。新交付必须使用 `networkPolicy.externalEgressRules`，同时指定调用组件、目标 CIDR、协议和端口；`egressCIDRs` 仅为旧配置兼容字段，会放行目标 CIDR 的全部端口，新部署禁止使用。Calico v3.32.1/Kind v1.32.2 已真实验证允许端口可达、同 CIDR 未授权端口与未授权组件不可达、入口 Namespace 隔离，证据见 `evidence/v7-network-policy-20260815.json`；客户目标 CNI 和最终 allowlist 仍需现场复验。
- 发布流水线生成 SPDX SBOM，以 Trivy 阻断可修复 High/Critical 漏洞，并用 Cosign OIDC 签名镜像。
- 日志使用 JSON；当前审计导出实现为带界限队列的 SIEM HTTPS JSON Webhook，支持 TLS、自定义 CA、从文件轮换 Bearer Token、有限指数退避重试以及丢弃/最终失败计数，尚未实现 syslog。自动验收使用真实临时 TLS 端点验证自定义 CA 握手、Token 热轮换和 HTTP 503 恢复；客户 SIEM 仍须现场联调。
- `deploy/production/diagnostics.sh` 提供最小脱敏诊断包：只收集组件镜像/状态、健康探针、迁移版本、Local Agent 聚合状态和部署文件哈希，不收集日志、`.env`、数据库业务内容、私钥、Keystore 或令牌；输出权限为 `0600`，包含 SHA 清单并执行敏感值扫描。仅系统管理员可见的“部署与升级”页面通过只读受 RBAC 和服务端用户类型双重保护的接口展示 API/System/Core/Builder gRPC 健康、数据库实际迁移、产品版本、部署模式和许可证公开状态；页面只复制诊断命令，不允许浏览器执行服务器 Shell。
- Local Agent 不接收入站连接，也不提供 shell/任意命令 RPC；任务协议只有注册、心跳、领取、续租、进度、完成/失败、轮换和 Drain。

## 故障排查

先运行 `diagnostics.sh OUTPUT.tar.gz` 收集版本、部署模式、组件 `/healthz`/`/readyz`、迁移版本和 Agent 协议聚合状态。日志与错误正文不自动进入诊断包；如需人工补充，必须先脱敏。禁止收集 `.env`、私钥、Keystore、API Key、Webhook Secret、支付 payload 或完整数据库。Agent 离线时暂停领取并等待租约回收，不手工修改 attempt/fencing 字段。

## 当前交付边界

Compose、Helm、配置 Schema、迁移 Job、备份恢复脚本、离线镜像脚本、离线许可证、Local Agent 客户安装/升级包、控制面协议、固定 APK 执行器和企业 Secret Resolver 已实现基础能力；全新 Private 与 Dedicated Compose 空数据卷安装、真实 Agent 临期证书自动轮换与吊销拒绝、Local Agent 真实 APK 执行中断恢复及离线升级回滚、kind/Helm 滚动升级与应用回滚已完成本地验收。正式交付前仍必须完成：客户 Linux/物理断网复验、完整业务备份恢复与 RPO/RTO 计时、三种 Artifact 控制面到 Agent 的真实 APK E2E、真实 AWS、发布签名流水线、目标网络策略和客户 SIEM 联调。当前状态以 [V7 企业交付验收报告](../../V7企业交付验收报告.md) 为准。

统一基础回归入口为 `deploy/acceptance/v1-v7-regression-base.sh`，要求 Node.js 20 或更高版本，并覆盖 11 个 Go 模块、统一前端类型检查、交付 Shell 语法、V4–V7 本地运行验收和限定规模并发性能冒烟。该脚本明确不替代 Android 授权真机、客户容量/峰值、APK/对象存储吞吐、长稳或外部系统验收。
