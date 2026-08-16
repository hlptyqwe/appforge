# AppForge 企业交付手册

## 交付模式

- Dedicated：单客户独立控制面、数据库、对象存储和 Builder 池，由平台运维。
- Private：全部组件位于客户环境，可使用 Compose 或 Helm，可断网安装。
- Hybrid：控制面可托管，APK、Keystore、客户对象存储和构建过程留在客户网络，Local Agent 仅主动出站。

## 安装

Compose：复制 `deploy/production/.env.example` 为 `.env`，生成随机 Secret、TLS 证书和 P-256 ECDSA Agent CA，将 `.env`、TLS 私钥、Agent CA 私钥和 SIEM Token 权限设为仅所有者可读写，再运行 `preflight.sh`；门禁会拒绝占位凭据、非 HTTPS、符号链接、证书/私钥不匹配、非 ECDSA Agent CA、即将过期的证书以及向 group/others 开放的私有文件。通过后再启动 `docker compose up -d`。生产镜像必须固定语义化标签和发布清单中的 digest。

Kubernetes：预先创建 MySQL、Redis、对象存储、内部 RPC、JWT、主密钥和 Agent CA Secret；已有 RWX 许可证状态 Claim 的根目录必须预置为 UID/GID `65532` 可写且不得向其他主体开放；填写 Helm values 后执行 `helm lint` 和 `helm upgrade --install --atomic`。Schema 会拒绝空镜像仓库、非 HTTPS 入口、单副本控制面和无版本镜像。本地 kind v1.32.2 已验证 `1.2.0 → 1.2.1 → 1.2.0` 滚动升级与应用回滚期间 API 连续可用、Schema 113 和 AIR_GAPPED 表可被回滚应用继续使用，客户交付仍需使用具有真实版本差异的正式镜像在目标 CSI 与 Ingress 上复验。

离线环境使用 `offline-bundle.sh` 在联网交付机导出 OCI 镜像包，在断网环境用 `offline-install.sh` 校验 SHA 后导入。正式 tag 流水线为 16 个 AppForge 发布镜像按 digest 生成 SPDX SBOM、标准化许可证清单、Trivy SARIF 和 Cosign 回验 JSON；其中 MySQL、etcd、MinIO 和 mc 是由工作流固定上游源码提交、升级已知漏洞依赖后构建的可追溯衍生镜像，不改变其上游许可证。随介质交付的 Redis、Alpine 两个原始第三方镜像必须解析为不可变 digest，并生成 SPDX、许可证清单和 Trivy SARIF。另有独立 `trivy fs` 门禁扫描仓库中的 Go module 与前端 manifest/lockfile，发现可修复 High/Critical 依赖漏洞会直接阻断发布；为避免未授权外传，该门禁不上传源码依赖 SBOM、许可证或漏洞明细，只把成功状态写入聚合签名 Manifest。镜像证据统一汇总为 `RELEASE-MANIFEST.json`、`SHA256SUMS`，再由工作流 OIDC 身份签名；Redis、Alpine 不冒充 AppForge 自签镜像，其来源由聚合签名清单锁定。工作流还会把 `release-security-vX.Y.Z` 与 `delivery-tools-vX.Y.Z` 打包为公开 GitHub Release 资产，生成外层 SHA 清单并用同一 OIDC 身份签名。联网打包机使用 `download-release-assets.sh` 匿名下载，先验证外层资产签名和哈希，再验证两个归档内的签名、固定文件集合、版本和仓库身份；`offline-bundle.sh VERSION RELEASE_SECURITY_DIRECTORY OUTPUT.tar` 随后从每个已签名索引解析明确目标平台的子 manifest digest，拉取并重新标记全部 18 个镜像，把 `PLATFORM`、`PLATFORM-IMAGES`（部署标签、平台子摘要、签名索引摘要）、完整安全证据、自包含 Admin API 模板、控制面部署配置、Local Agent 客户安装包、`licensectl`/`appforgectl` 和企业文档一并写入离线介质。缺失、额外、跨版本、跨仓库、目标平台不存在或 SHA 被篡改的证据均会被拒绝。

联网打包示例：

```bash
export APPFORGE_COSIGN_BINARY=/usr/local/bin/cosign
deploy/production/download-release-assets.sh \
  1.2.6 OWNER/REPOSITORY release-v1.2.6

export APPFORGE_IMAGE_REGISTRY=ghcr.io/OWNER/appforge
export APPFORGE_OFFLINE_PLATFORM=linux/amd64
export APPFORGE_LICENSECTL_BINARY=$PWD/release-v1.2.6/delivery-tools-v1.2.6/licensectl-linux-amd64
export APPFORGE_APPFORGECTL_BINARY=$PWD/release-v1.2.6/delivery-tools-v1.2.6/appforgectl-linux-amd64
export APPFORGE_RELEASE_CERTIFICATE_IDENTITY=https://github.com/OWNER/REPOSITORY/.github/workflows/release-security.yml@refs/tags/v1.2.6
deploy/production/offline-bundle.sh \
  1.2.6 release-v1.2.6/release-security-v1.2.6 appforge-1.2.6-offline.tar

deploy/acceptance/v7-formal-offline-media.sh \
  appforge-1.2.6-offline.tar 1.2.6 ghcr.io/OWNER/appforge formal-offline-report.json

export APPFORGE_BASE_RELEASE_CERTIFICATE_IDENTITY=https://github.com/OWNER/REPOSITORY/.github/workflows/release-security.yml@refs/tags/v1.2.5
export APPFORGE_TARGET_RELEASE_CERTIFICATE_IDENTITY=https://github.com/OWNER/REPOSITORY/.github/workflows/release-security.yml@refs/tags/v1.2.6
deploy/acceptance/v7-formal-offline-upgrade.sh \
  appforge-1.2.5-offline.tar 1.2.5 \
  appforge-1.2.6-offline.tar 1.2.6 \
  ghcr.io/OWNER/appforge formal-offline-upgrade-report.json
```

不得使用本地伪造证据目录替代真实 tag Release 资产。当前正式容器镜像发布平台为 `linux/amd64`；`appforgectl`/`licensectl` 同时提供 AMD64 与 ARM64，但在正式镜像索引增加 ARM64 前，生成 `linux/arm64` 介质会按设计失败。断网安装后，原始证据位于 `security/`，校验器位于 `bin/validate-release-evidence`；介质进入断网区前还应按交付制度校验外部签名和介质哈希。`v1.2.6` 已完成真实 tag、OIDC、Cosign、Trivy、SBOM、许可证、聚合签名和公开 Release 流水线，22 个任务全部成功；四项公开资产已通过匿名下载、外层与内层 SHA/Sigstore 身份和 Linux CLI 权限验证，证据见 `evidence/v7-public-release-v1.2.6-20260816.json`。由这些资产生成的正式介质大小为 3,324,800,000 字节，SHA-256 为 `a88b792badcf39bd1522fd1e1923eb8226c0fa8f51e5ebb5cc5e9ecf9f46c306`，已完成 18 镜像 `linux/amd64` 本地断网全新安装，证据见 `evidence/v7-formal-offline-media-v1.2.6-20260816.json`。`v1.2.5` 与 `v1.2.6` 两套正式介质还完成了本地 `1.2.5 → 1.2.6 → 1.2.5` 禁拉取升级/回滚，16/16 个 AppForge 平台镜像摘要不同，证据见 `evidence/v7-formal-offline-upgrade-v1.2.5-v1.2.6-20260817.json`。客户离线介质仍必须使用真实 tag 资产在联网交付机独立复验，并完成物理介质交接和客户现场升级记录。

Hybrid/Private 客户构建节点使用 `deploy/local-agent` 交付包。该包提供非 root、只读根文件系统、无入站端口的 Compose，注册码通过标准输入传递，Agent 身份与签名 Secret 分别保存在 Docker 私有卷，并提供健康检查、Drain 后在线/已导入镜像离线升级和失败自动恢复旧镜像。详细步骤见 [Local Agent 客户侧安装与升级](../../deploy/local-agent/README.md)。

Hybrid 的客户 APK/Keystore 上传不需要第二套代理前端：Local Agent 本地导入命令直接写客户 S3/MinIO/OSS，再通过 mTLS 只登记引用、大小、SHA 和对象 ID。接口、显式存储归属、前缀隔离和验收门禁以 [CUSTOMER_STORAGE 实施契约](./customer-storage-contract.md) 为准；临时 MinIO 合成数据 E2E 已通过，真实 S3/阿里云 OSS 仍需客户环境复验。

完全断网的客户构建节点使用签名任务包和结果包，不启动在线 Agent loop。控制面导出、离线固定构建、双向签名、防重放和结果导入必须遵守 [AIR_GAPPED Artifact 实施契约](./air-gapped-artifact-contract.md)。Local Agent 管理页按权限提供“锁定任务并导出 ZIP、查询包状态、上传并导入结果 ZIP”，浏览器不会连接离线 Agent；介质搬运和 Agent 端构建仍由受控离线流程完成。平台端与租户端导出/导入/查询使用独立权限，导入角色还需现有 `core:storage:upload`；迁移不会给已有角色自动授权，交付方必须创建专用角色。Schema 113 已是 1.2.x 生产默认，并通过空库、112→113、幂等和已有角色零自动授权验收。

## 升级与回滚

1. 阅读 `version-compatibility.md`，确认版本跨度和 Agent 协议窗口。
2. 执行生产配置 preflight，检查磁盘至少为当前数据库与对象数据的两倍。
3. 执行 `backup.sh`，在独立环境验证校验和与恢复。
4. 先运行 expand 迁移，再滚动升级无状态 API/RPC，最后升级 Worker 与 Agent。
5. 验证 `/healthz`、`/readyz`、迁移版本、构建领取和审计导出。
6. 应用故障且 Schema 兼容时回滚镜像；不可逆迁移必须恢复升级前备份到独立环境。

## 备份恢复与灾备

参考目标为 RPO 15 分钟、RTO 120 分钟；企业可在 values 或 `.env` 中收紧。生产 Compose 已为 MySQL 启用 ROW binlog；对象存储仍应启用版本化与跨区域复制，etcd 应定时快照。`backup.sh` 在冻结写入方后生成带 `SOURCE_LOG_FILE/SOURCE_LOG_POS` 的 MySQL 一致性导出、同点 binlog 坐标、etcd 快照、对象数据和覆盖全部清单文件的 SHA-256；`restore.sh` 需要显式确认目标目录并停止写服务。

PITR 使用随正式发布和离线 OCI 包交付的同版本 `mysql-binlog-tools` 镜像，不在客户现场安装软件。先执行 `archive-binlogs.sh BASE_BACKUP_DIRECTORY OUTPUT_DIRECTORY`：脚本验证基线 SHA，切换当前 binlog，只归档从基线坐标开始的已关闭日志，并生成有序 `BINLOGS` 与独立 SHA 清单。恢复时设置精确确认值 `APPFORGE_PITR_CONFIRM='BASE_ABSOLUTE_PATH|ARCHIVE_ABSOLUTE_PATH|YYYY-MM-DD HH:MM:SS'`，再执行 `pitr-restore.sh BASE_BACKUP_DIRECTORY BINLOG_ARCHIVE_DIRECTORY 'YYYY-MM-DD HH:MM:SS'`；截止时间按 UTC 解释。脚本先完成数据库/etcd/对象基线恢复并保持业务服务停止，再从基线位置按序回放日志，成功后才启动全部服务。基线与 binlog 归档必须复制到独立故障域并按保留策略防止 MySQL 日志过期早于归档。

生产 MinIO 初始化会为 `appforge` 桶启用版本化。配置 `.env` 中的 `APPFORGE_REPLICA_ENDPOINT`、受限复制账号、目标桶、规则 ID 与同步策略后，先通过 `preflight.sh` 的 HTTPS/凭据/桶名门禁，再执行 `configure-object-replication.sh`。脚本只管理指定规则 ID，为源桶和目标桶启用版本化，并配置已有对象、元数据、删除标记与版本化删除的服务端复制；默认异步，只有明确设置 `APPFORGE_REPLICA_SYNC=true` 才同步复制。目标凭据必须只允许所需桶和复制操作。2026-08-15 的合成验收使用两个独立 MinIO 卷，复制两个版本和删除标记后销毁源容器/卷，再从副本版本历史恢复最新对象；SHA 全部一致、复制/恢复均为 1 秒且临时资源零残留，证据见 `evidence/v7-object-replication-20260815.json`。同机双卷不代表跨区域故障域，客户仍须在实际区域、网络、KMS、保留和带宽条件下复验。

恢复后对比：租户数、任务状态摘要、对象 ID/Key/SHA、操作审计摘要和最高迁移版本。恢复演练不得覆盖生产环境，应使用独立网络、数据库和对象桶。`deploy/acceptance/v7-backup-restore.sh` 已验证 MySQL、etcd 和对象卷的基础破坏后恢复机制，并对恢复点间隔与破坏到恢复核验完成进行计时；2026-08-15 隔离合成数据结果为 RPO 2 秒、RTO 9 秒，见 `evidence/v7-synthetic-dr-20260815.json`。`deploy/acceptance/v7-appforge-schema112-dr.sh` 保留历史 1.1.x/Schema 112 兼容恢复证据，严格排除 Schema 113，RPO 2 秒、RTO 10 秒，见 `evidence/v7-appforge-schema112-dr-20260815.json`。当前 `deploy/acceptance/v7-appforge-schema113-dr.sh` 使用 1.2.x/Schema 113 和代表性合成租户、应用、版本、渠道、签名配置、构建任务、操作审计、AIR_GAPPED 已导入状态及六对象引用链；销毁整个隔离数据库、etcd 前缀和精确对象目录后，恢复的 58 张 AppForge 表、业务引用和对象字节摘要完全一致，RPO 2 秒、RTO 11 秒，0600 报告见 `evidence/v7-appforge-schema113-dr-20260816.json`。`deploy/acceptance/v7-mysql-pitr.sh` 还在唯一临时栈中验证了 dump 坐标、binlog 归档 SHA、有意删除测试表后的 UTC 截止时间恢复，以及“截止前事件存在、截止后事件不存在”；RTO 7 秒且资源零残留，证据见 `evidence/v7-mysql-pitr-20260815.json`。`deploy/acceptance/v7-object-replication.sh` 验证了独立源/副本卷的版本化复制、删除标记和源卷销毁后的副本恢复。正式交付仍必须使用生产规模业务备份执行业务摘要核对、真实跨区域副本恢复和客户环境计时。

## 安全与运维

- 所有容器以非 root、只读根文件系统、drop capabilities 和 no-new-privileges 运行。
- 生产 Secret 只由 Kubernetes Secret、Vault/KMS 或外部 Secret Operator 注入，不写入 values、Compose 文件、日志和诊断包。
- NetworkPolicy 默认拒绝，仅放行组件间、DNS、入口 Namespace 和明确外部依赖出站。新交付必须使用 `networkPolicy.externalEgressRules`，同时指定调用组件、目标 CIDR、协议和端口；`egressCIDRs` 仅为旧配置兼容字段，会放行目标 CIDR 的全部端口，新部署禁止使用。Calico v3.32.1/Kind v1.32.2 已真实验证允许端口可达、同 CIDR 未授权端口与未授权组件不可达、入口 Namespace 隔离，证据见 `evidence/v7-network-policy-20260815.json`；客户目标 CNI 和最终 allowlist 仍需现场复验。
- 受限网络环境可启用内置 `egress-proxy`。它仅接受 Allowlist 中的 HTTPS CONNECT，拒绝普通 HTTP 和未登记目标；Compose 只有代理连接非 internal 出口网络，Helm 要求代理专属 CIDR/端口 NetworkPolicy。远程 APK 签名和 Webhook 为保护 APK 与 SSRF 边界而显式不继承通用代理，详细配置和直连要求见 [受限网络出口与企业代理契约](./restricted-egress-proxy.md)。本地合成 TLS E2E 证据见 `evidence/v7-egress-proxy-20260816.json`，客户最终域名/IP、CNI、DNS、防火墙和容量仍需现场联调。
- 发布流水线对 16 个 AppForge 发布镜像和两个原始第三方运行镜像生成 SPDX SBOM、独立许可证清单，并以 Trivy 阻断可修复 High/Critical 漏洞；16 个 AppForge 发布镜像另用 Cosign OIDC 逐镜像签名和回验。仓库源码依赖再由 Trivy 文件系统模式独立阻断，且不导出依赖明细。全部 18 个 digest、镜像证据和源码门禁成功状态由同一工作流身份签署聚合清单，离线包生成器必须先验证该聚合签名并按清单 digest 拉取镜像。
- 日志使用 JSON；当前审计导出实现为带界限队列的 SIEM HTTPS JSON Webhook，支持 TLS、自定义 CA、从文件轮换 Bearer Token、有限指数退避重试以及丢弃/最终失败计数，尚未实现 syslog。自动验收使用真实临时 TLS 端点验证自定义 CA 握手、Token 热轮换和 HTTP 503 恢复；客户 SIEM 仍须现场联调。
- API、RPC 和纯 Worker 统一通过 `APPFORGE_PROMETHEUS_*` 暴露独立指标端口，并通过 `APPFORGE_OTLP_ENDPOINT`/`APPFORGE_OTLP_SAMPLER` 导出 OTLP/HTTP Trace。生产 Compose 仅在后端网络暴露指标；Helm 默认只允许 `monitoring` Namespace 抓取，可通过 `observability.prometheusNamespaceLabels` 精确替换。生产 OTLP 必须使用无凭据的 HTTPS URL，认证 Header 只允许从受保护运行配置注入；本地合成验收证据见 `evidence/v7-observability-20260815.json`，客户 Collector、证书和容量仍需现场联调。
- `deploy/production/diagnostics.sh` 提供最小脱敏诊断包：只收集组件镜像/状态、健康探针、迁移版本、Local Agent 聚合状态和部署文件哈希，不收集日志、`.env`、数据库业务内容、私钥、Keystore 或令牌；输出权限为 `0600`，包含 SHA 清单并执行敏感值扫描。仅系统管理员可见的“部署与升级”页面通过只读受 RBAC 和服务端用户类型双重保护的接口展示 API/System/Core/Builder gRPC 健康、数据库实际迁移、产品版本、部署模式和许可证公开状态；页面只复制诊断命令，不允许浏览器执行服务器 Shell。
- Local Agent 不接收入站连接，也不提供 shell/任意命令 RPC；任务协议只有注册、心跳、领取、续租、进度、完成/失败、轮换和 Drain。

## 故障排查

先运行 `diagnostics.sh OUTPUT.tar.gz` 收集版本、部署模式、组件 `/healthz`/`/readyz`、迁移版本和 Agent 协议聚合状态。日志与错误正文不自动进入诊断包；如需人工补充，必须先脱敏。禁止收集 `.env`、私钥、Keystore、API Key、Webhook Secret、支付 payload 或完整数据库。Agent 离线时暂停领取并等待租约回收，不手工修改 attempt/fencing 字段。

## 当前交付边界

Compose、Helm、配置 Schema、迁移 Job、备份恢复脚本、离线镜像脚本、离线许可证、Local Agent 客户安装/升级包、控制面协议、固定 APK 执行器和企业 Secret Resolver 已实现基础能力；全新 Private 与 Dedicated Compose 空数据卷安装、真实 Agent 临期证书自动轮换与吊销拒绝、Local Agent 真实 APK 执行中断恢复及离线升级回滚、三种 Artifact 模式的合成真实构建 E2E、当前 Schema 113 灾备、kind/Helm 1.2.x 滚动升级与应用回滚已完成本地验收。本地模拟断网已通过预载镜像、禁止拉取、双 internal 网络、内部 TLS/API 登录、外部 HTTPS 拒绝及严格 Schema 113 门禁；Schema 113 的空库安装、112→113 升级、幂等、数据保持和已有角色零自动授权证据见 `evidence/v7-schema113-production-20260816.json`。npm 官方生产及全部依赖审计均为零漏洞，证据见 `evidence/v7-npm-audit-20260816.json`。发布安全证据契约已完成 16 个自建签名加两个原始第三方镜像的合成结构、许可证清单、源码依赖无明细导出门禁、SHA 防篡改与离线介质落盘验收，证据见 `evidence/v7-release-evidence-contract-20260816.json`；`v1.2.6` 的真实远端 tag/OIDC/漏洞库、公开 Release、匿名双层签名验证、正式介质本地断网全新安装以及 `v1.2.5 → v1.2.6 → v1.2.5` 正式版本差异升级/回滚均已通过，证据见 `evidence/v7-public-release-v1.2.6-20260816.json`、`evidence/v7-formal-offline-media-v1.2.6-20260816.json` 与 `evidence/v7-formal-offline-upgrade-v1.2.5-v1.2.6-20260817.json`。正式交付前仍必须完成：客户 Linux/物理断网介质及现场离线升级复验、生产规模业务备份恢复与客户 RPO/RTO、真实 AWS/S3/OSS、目标网络策略和客户 SIEM 联调。当前状态以 [V7 企业交付验收报告](../../V7企业交付验收报告.md) 为准。

客户 HSM/远程 APK 签名必须遵守 [远程 APK 签名与 HSM 接入契约](./remote-apk-signing-contract.md)。仓库已完成管理 API、Schema 112 兼容持久化、创建时身份验证、Core 任务快照、Builder 远程签名分支、能力调度门禁、最终 APK 证书校验，以及协议级和完整任务级合成 E2E；该证据仍不等于真实 HSM、不可导出密钥、PIN/会话、HA、审计、限流或性能已经验收。

统一基础回归入口为 `deploy/acceptance/v1-v7-regression-base.sh`，要求 Node.js 20 或更高版本，并覆盖 11 个 Go 模块、统一前端类型检查、交付 Shell 语法、V4–V7 本地运行验收和限定规模并发性能冒烟。`deploy/acceptance/v7-artifact-capacity.sh` 另行覆盖合成真实 APK 并发构建、本地 MinIO 双向吞吐、逐对象 SHA-256 和短时 API 稳定性，并生成 0600 JSON 证据。两者都不替代 Android 授权真机、客户目标容量/峰值、生产规模 APK/对象存储吞吐、小时/天级长稳或外部系统验收。
