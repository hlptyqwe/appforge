# 受限网络出口与企业代理契约

## 1. 目标与边界

AppForge 的 Private/Hybrid 控制面默认不应具有无限制互联网出口。仓库提供 `egress-proxy`：一个非 root、只读根文件系统、带连接上限的 HTTPS CONNECT-only 代理。它只接受交付管理员预先登记的 `host:port` 或 `*.domain:port`，拒绝普通 HTTP 转发、未登记目标、URL、路径、凭据和全局通配符。

该组件是服务端受限网络出口，不是客户上传 APK 的“代理前端”。客户 APK 的浏览器上传、`CUSTOMER_STORAGE` Local Agent 导入和 `AIR_GAPPED` 介质流程保持不变。

## 2. 调用边界

通用代理环境只供明确允许继承系统代理的 TLS 客户端使用，包括代码平台 OAuth/API、支付、Vault/AWS、客户对象存储、OTLP 和 SIEM。SIEM HTTPS exporter 继承标准环境代理；RFC5424 Syslog TLS 使用同一 `HTTPS_PROXY` 建立无凭据 HTTP CONNECT 隧道，不允许绕过受限出口。

以下两类流量必须绕过通用代理：

- `REMOTE_APK_SIGNER` 继续使用独立 mTLS Transport 且 `Proxy=nil`，避免未签名 APK 被非预期代理接收。
- Webhook Worker 继续使用独立 DNS/IP 校验和 `Proxy=nil`，避免代理解析绕过 SSRF 防护。

Kubernetes 中如需这两类直连，必须为对应 `builder`/`builder-worker` 或 `webhook-worker` 配置独立的 `networkPolicy.externalEgressRules`。Compose 基础部署不会把应用容器连接到出口网络；客户应把 HSM/Webhook 受控网关接入 `appforge-backend`，或在宿主机防火墙具有等价目标级阻断后另行评审直连网络，不能为了联通而把整个后端网络改为非 internal。

## 3. Allowlist 格式

每行一条，空行和 `#` 注释忽略：

```text
siem.customer.example:443
syslog.customer.example:6514
collector.customer.example:4318
*.s3.customer.example:443
[2001:db8::20]:443
```

通配符只允许最左侧单标签。`*.example.org` 不匹配裸 `example.org`。代理按 CONNECT 请求中的主机和端口先检查规则，再解析一次 DNS 并直接拨号到已解析 IP，避免授权与连接之间再次解析。Allowlist 最多 1024 条；生产文件不得是符号链接或可被 group/others 修改。

## 4. Compose

1. 创建 `secrets/egress-allowlist.txt`，填写客户批准的真实目标并设置不可被 group/others 修改。
2. 设置 `APPFORGE_EGRESS_PROXY_ENABLED=true`、`APPFORGE_EGRESS_PROXY_URL=http://egress-proxy:3128` 和 Allowlist 路径。
3. `APPFORGE_EGRESS_NO_PROXY` 必须保留 MySQL、Redis、etcd、MinIO 和内部 RPC 名称。
4. 运行 `preflight.sh`，再使用 `docker compose --profile egress up -d`。

应用容器仍只连接 `appforge-backend` internal 网络；仅 `egress-proxy` 同时连接 internal 后端和 `appforge-egress`。未启用 profile 时代理不存在且应用外部 HTTPS 继续失败。正式离线包和发布签名矩阵包含 `egress-proxy` 镜像。

## 5. Helm

启用 `egressProxy.enabled` 时必须同时满足：

- `networkPolicy.enabled=true`；
- `egressProxy.allowlist` 非空且不含示例占位域名；
- 至少一条 `component: egress-proxy` 的 `externalEgressRules`，按客户目标 CIDR、协议和端口放行；
- `global.offline=false`。

应用 Pod 只能通过集群内 `appforge-egress-proxy:3128` 发起通用 HTTPS CONNECT；默认拒绝策略阻止应用绕过代理直连。NetworkPolicy 仍是 IP/CIDR 控制，域名规则由代理二次收窄；客户必须在目标 CNI 和实际 DNS 上复验两层规则。

## 6. 验收边界

`deploy/acceptance/v7-egress-proxy.sh` 使用临时本机 TLS 端点验证允许目标可达、未登记 CONNECT 返回 403、普通 HTTP 返回 405、SIEM 可继承代理以及远程 APK 签名仍不继承代理，并生成 0600 机器证据。`deploy/acceptance/v7-syslog-tls.sh` 进一步验证 RFC5424 Syslog TLS 的真实 CONNECT 隧道、两次独立 TLS 连接和凭据型代理拒绝。测试不访问客户数据、生产凭据或真实外部域名，也不替代客户最终域名/IP、代理容量、DNS、CNI 和防火墙联调。
