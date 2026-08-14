# AppForge 企业交付手册

## 交付模式

- Dedicated：单客户独立控制面、数据库、对象存储和 Builder 池，由平台运维。
- Private：全部组件位于客户环境，可使用 Compose 或 Helm，可断网安装。
- Hybrid：控制面可托管，APK、Keystore、客户对象存储和构建过程留在客户网络，Local Agent 仅主动出站。

## 安装

Compose：复制 `deploy/production/.env.example` 为 `.env`，生成随机 Secret 和 TLS/Agent CA，运行 `preflight.sh`，确认无开发凭据后再启动 `docker compose up -d`。生产镜像必须固定语义化标签和发布清单中的 digest。

Kubernetes：预先创建 MySQL、Redis、对象存储、内部 RPC、JWT、主密钥和 Agent CA Secret；填写 Helm values 后执行 `helm lint` 和 `helm upgrade --install --atomic`。Schema 会拒绝空镜像仓库、非 HTTPS 入口、单副本控制面和无版本镜像。

离线环境使用 `offline-bundle.sh` 在联网交付机导出 OCI 镜像包，在断网环境用 `offline-install.sh` 校验 SHA 后导入。离线包、SBOM、漏洞报告和签名证明应归档在同一发布目录。

## 升级与回滚

1. 阅读 `version-compatibility.md`，确认版本跨度和 Agent 协议窗口。
2. 执行生产配置 preflight，检查磁盘至少为当前数据库与对象数据的两倍。
3. 执行 `backup.sh`，在独立环境验证校验和与恢复。
4. 先运行 expand 迁移，再滚动升级无状态 API/RPC，最后升级 Worker 与 Agent。
5. 验证 `/healthz`、`/readyz`、迁移版本、构建领取和审计导出。
6. 应用故障且 Schema 兼容时回滚镜像；不可逆迁移必须恢复升级前备份到独立环境。

## 备份恢复与灾备

参考目标为 RPO 15 分钟、RTO 120 分钟；企业可在 values 或 `.env` 中收紧。MySQL 应启用 binlog/PITR，对象存储启用版本化与跨区域复制，etcd 定时快照。`backup.sh` 生成 MySQL 一致性导出、etcd 快照、对象数据和 SHA 清单；`restore.sh` 需要显式确认目标目录并停止写服务。

恢复后对比：租户数、任务状态摘要、对象 ID/Key/SHA、操作审计摘要和最高迁移版本。恢复演练不得覆盖生产环境，应使用独立网络、数据库和对象桶。

## 安全与运维

- 所有容器以非 root、只读根文件系统、drop capabilities 和 no-new-privileges 运行。
- 生产 Secret 只由 Kubernetes Secret、Vault/KMS 或外部 Secret Operator 注入，不写入 values、Compose 文件、日志和诊断包。
- NetworkPolicy 默认拒绝，仅放行组件间、DNS、明确对象存储和代码平台出站。
- 发布流水线生成 SPDX SBOM，以 Trivy 阻断可修复 High/Critical 漏洞，并用 Cosign OIDC 签名镜像。
- 日志使用 JSON，审计可通过 SIEM HTTPS/syslog 导出；诊断包必须先执行脱敏。
- Local Agent 不接收入站连接，也不提供 shell/任意命令 RPC；任务协议只有注册、心跳、领取、续租、进度、完成/失败、轮换和 Drain。

## 故障排查

先收集版本、部署模式、组件 `/healthz`/`/readyz`、最高迁移版本、Agent 协议和最近脱敏错误摘要。禁止收集 `.env`、私钥、Keystore、API Key、Webhook Secret、支付 payload 或完整数据库。Agent 离线时暂停领取并等待租约回收，不手工修改 attempt/fencing 字段。
