# V7 客户现场验收与双签证据契约

## 1. 目的与边界

V7 剩余项必须在客户控制的目标环境执行，不能由仓库夹具、公开 Runner、Kind、临时 MinIO 或本地 Docker 代替。本契约把外部验收固定为 8 个门禁，统一版本、Schema、Agent 协议、脱敏、SHA-256 和客户/AppForge 双签格式，使现场结果可机器校验和归档。

双签只证明双方批准同一组字节；它不自动证明测试真实发生。客户必须保留变更单、介质交接、基础设施审计和原始运行日志，AppForge 交付负责人必须复核后才能签署。任何门禁存在开放问题、使用 `fixture`、缺少目标基础设施或指标未达到双方事先批准的阈值时，都不得生成 `result=passed`。

## 2. 固定 8 门禁

| gateId | V7 严格矩阵 | 目标环境必须证明 |
| --- | --- | --- |
| `customer-object-storage` | 7、9 | 真实 AWS S3 与阿里云 OSS；登记前缀最小权限；工作负载身份或短期凭据；合成对象 Put/Stat/完整 Get SHA/Delete/NotFound；既有对象不变且零残留 |
| `physical-air-gap` | 7、10 | 已签名正式介质交接、物理断网、正式证书、全新安装、升级、回滚和 AIR_GAPPED 最小 RBAC |
| `customer-kubernetes` | 2、11、12 | 客户 CNI、CSI、Ingress、私有仓库、镜像签名、Schema 113、滚动升级/应用回滚、零探测失败 |
| `remote-signing-hsm` | 9、11 | 真实 HSM 不可导出密钥、PIN/会话、HA、审计关联、限流、性能和最终 APK 证书；控制面零密钥材料 |
| `observability-egress` | 11 | 最终 allowlist、DNS、防火墙、CONNECT 代理容量、Prometheus、OTLP、SIEM 解析/路由/留存/告警 |
| `disaster-recovery` | 3、12 | 经授权隔离的生产规模数据、跨故障域副本、MySQL PITR、对象版本恢复、etcd、业务摘要、客户 RPO/RTO |
| `capacity-soak` | 13 | 客户批准的峰值模型、至少 86,400 秒、API/APK/对象零错误、核心容器零重启且吞吐阈值全部达成 |
| `android-physical-devices` | 13 | 客户批准的物理真机矩阵全部覆盖旧版安装/启动/UI、同签名原位升级、首次安装时间保持和新版启动 |

门禁之间可以在同一维护窗口执行，但必须输出 8 个独立 JSON。客户 Kubernetes、灾备或容量测试不能因为另一个门禁通过而省略。

## 3. 证据目录

签署完成后的目录必须且只能包含：

```text
CUSTOMER-SITE-MANIFEST.json
SHA256SUMS
SHA256SUMS.customer.sig
SHA256SUMS.appforge.sig
android-physical-devices.json
capacity-soak.json
customer-kubernetes.json
customer-object-storage.json
disaster-recovery.json
observability-egress.json
physical-air-gap.json
remote-signing-hsm.json
```

每个门禁文件使用公共信封：

```json
{
  "schemaVersion": 1,
  "evidenceType": "v7-customer-site-gate",
  "gateId": "customer-object-storage",
  "result": "passed",
  "runId": "customer-change-20260818-storage",
  "environmentKind": "customer-test",
  "siteFingerprint": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
  "releaseVersion": "1.2.7",
  "targetSchemaVersion": "20260815_113_v7_air_gapped",
  "agentProtocol": 3,
  "gitCommit": "0123456789abcdef0123456789abcdef01234567",
  "startedAt": "2026-08-18T00:00:00Z",
  "finishedAt": "2026-08-18T01:00:00Z",
  "verified": ["由固定门禁验证器要求的项目"],
  "metrics": {},
  "openFindings": [],
  "dataPolicy": {
    "credentialsIncluded": false,
    "rawCustomerDataIncluded": false,
    "executionApprovalRef": "CUSTOMER-CHANGE-20260818"
  }
}
```

`environmentKind` 只允许 `customer-test` 或 `production`。所有文件必须使用同一 `siteFingerprint`、正式版本、Schema 113、协议3和 Git commit。证据不得包含 endpoint/bucket/prefix 原值、URL、密码、Token、访问密钥、私钥、Keystore Secret、原始客户数据或其他可复用凭据；只能保存目标、设备和对象的单向 SHA-256 指纹以及不含身份信息的计数/耗时。

## 4. 指标要求

- 对象存储的 `metrics.providerResults` 必须同时且只覆盖 `aws-s3` 与 `aliyun-oss`，两者都确认最小前缀、完整回读、删除 NotFound、既有对象不变和零合成残留。
- Kubernetes 的升级与回滚探测失败数必须为 0，回滚后 Ready API 副本至少 2。
- HSM 请求失败数必须为 0，P99 和 HA 切换时间不得超过双方预先批准的目标。
- 灾备的实际 RPO/RTO 不得超过客户目标，恢复业务对象数必须大于 0，业务摘要差异必须为 0。
- 容量门禁必须持续至少 86,400 秒；API、APK 构建和对象往返均有实际负载且错误为 0，核心容器重启为 0，观测吞吐不得低于客户批准阈值。
- Android `approvedTargets` 与 `results` 必须一一对应；每个结果使用设备指纹而非序列号，并声明 `physicalDevice=true`，全部安装、启动、UI 和升级检查为真。

固定 `verified` 项和字段级机器约束以离线介质中的 `bin/validate-customer-site-evidence` 为权威。不要复制旧证据后只修改时间或环境名称。

## 5. 汇总、签署和验证

先用离线介质中的初始化器生成权限为 `0700/0600` 的待执行模板。它只接受 1.2.x、40 位 Git commit、脱敏站点指纹和真实客户环境，输出 `metadata.json` 与 `gates/` 下固定 8 个 `result=pending` 文件；模板明确带有 `pending-customer-execution`，不能被汇总器接受：

```bash
bin/init-customer-site-evidence \
  /secure/appforge-site-work \
  1.2.7 \
  0123456789abcdef0123456789abcdef01234567 \
  0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef \
  customer-test
```

在真实现场执行后，把 `gates/` 中每项的时间、精确指标和固定 `verified` 集合替换为实际结果，将 `result` 改为 `passed` 并清空 `openFindings`。不得仅修改状态而不保留原始运行记录。然后填写不含客户名称、URL或凭据的 `metadata.json`：

```json
{
  "environmentKind": "customer-test",
  "siteFingerprint": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
  "releaseVersion": "1.2.7",
  "targetSchemaVersion": "20260815_113_v7_air_gapped",
  "agentProtocol": 3,
  "gitCommit": "0123456789abcdef0123456789abcdef01234567",
  "generatedAt": "2026-08-18T02:00:00Z",
  "customerApproval": {
    "approvalRef": "CUSTOMER-CHANGE-20260818",
    "approverRole": "customer-change-owner"
  },
  "appforgeApproval": {
    "approvalRef": "APPFORGE-DELIVERY-20260818",
    "approverRole": "appforge-delivery-owner"
  }
}
```

在受控交付机生成待签名目录：

```bash
bin/assemble-customer-site-evidence \
  /secure/appforge-site-work/gates \
  /secure/appforge-site-evidence \
  /secure/appforge-site-work/metadata.json \
  /secure/customer-approval.pub.pem \
  /secure/appforge-approval.pub.pem
```

客户和 AppForge 分别使用受 OpenSSL `dgst -sha256` 支持的 RSA/ECDSA PEM 批准密钥，在自己的签名控制域对同一 `SHA256SUMS` 生成 detached signature。传给汇总器和校验器的必须是对应 PEM 公钥的同一文件字节；私钥不得进入交付包、证据目录、工单或另一方控制域：

```bash
openssl dgst -sha256 -sign /customer-controlled/customer-approval.key.pem \
  -out /secure/appforge-site-evidence/SHA256SUMS.customer.sig \
  /secure/appforge-site-evidence/SHA256SUMS

openssl dgst -sha256 -sign /appforge-controlled/appforge-approval.key.pem \
  -out /secure/appforge-site-evidence/SHA256SUMS.appforge.sig \
  /secure/appforge-site-evidence/SHA256SUMS
```

最终验证：

```bash
bin/validate-customer-site-evidence \
  /secure/appforge-site-evidence \
  /secure/customer-approval.pub.pem \
  /secure/appforge-approval.pub.pem
```

初始化器拒绝覆盖已有目录；汇总器会在签名前拒绝 pending、开放问题和上下文漂移，并在失败时原子清理新建的半成品目录。最终验证器会拒绝文件缺失/多余、符号链接、摘要篡改、签名或公钥不匹配、版本/Schema/协议漂移、`fixture` 冒充客户环境、开放问题、敏感字段、未脱敏 URL、门禁缺项和指标不足。只有验证成功且原始现场记录完成组织归档后，才可逐项更新 V7 严格矩阵；不得仅凭总清单中的 `result` 修改项目状态。

## 6. 已有执行入口

- 客户 S3/OSS 前置探针：`deploy/local-agent/customer-storage-probe.sh`；正式证据必须标记 `customer-test`，之后仍需完成真实控制面登记和 APK 构建。
- AIR_GAPPED、正式介质与本地基线：交付包中的 `delivery-guide.md`、`air-gapped-artifact-contract.md` 和正式离线介质校验器。
- 客户容量：`v7-api-soak.sh` 与 `v7-artifact-soak.sh` 支持 `environmentKind=customer-test`，持续时间和阈值必须按本契约提高到客户批准值。
- 灾备：随介质交付的 `backup.sh`、`restore.sh`、`archive-binlogs.sh`、`pitr-restore.sh` 和对象复制配置器。
- Kubernetes、HSM、可观测性与物理真机应由客户变更系统编排；只把脱敏结果写入上述固定 JSON，不把 kubeconfig、证书私钥、HSM PIN、SIEM Token、设备序列号或生产数据导出到证据包。
