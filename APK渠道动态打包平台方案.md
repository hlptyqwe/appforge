# APK 渠道动态打包平台方案

> 本文档描述产品方向和总体方案。V1 的双端边界、文件上传、对象安全、Builder 执行协议和验收标准，以 [《AppForge V1 功能闭环与实施规格》](./V1功能闭环实施规格.md) 为准。
>
> V1–V7 的实施顺序和状态见 [《AppForge V1–V7 产品实施路线图》](./V1-V7产品实施路线图.md)；V2–V7 的工程边界与验收标准以路线图链接的对应阶段规格为准。

## 1. 项目定位

该平台面向 Android APK 官网分发场景，核心目标是：

- 推广后台可随时新增渠道
- 每个渠道拥有唯一 `channelCode`
- 每个渠道对应一个独立推广链接
- 用户通过推广链接下载 APK
- APK 内置对应的 `channelCode`
- 用户安装并首次打开 App 后，上报 `channelCode`
- 后台根据渠道统计安装、注册、付费等转化数据
- 后续可扩展为白标 App、私有分发、企业私有部署

平台不建议只定位为“APK 动态打包工具”，而应定位为：

> Android 渠道分发 + 动态打包 + 安装归因 + 白标配置 + 版本管理平台

---

## 2. 核心业务流程

```text
推广后台创建渠道
        ↓
生成唯一 channelCode
        ↓
创建 Build Task
        ↓
Builder 获取任务
        ↓
将 channelCode 写入 APK
        ↓
Gradle Release Build
        ↓
使用正式签名签名
        ↓
上传 S3 / MinIO / OSS
        ↓
生成渠道推广链接
        ↓
用户下载 APK
        ↓
安装并首次启动
        ↓
App 读取 channelCode
        ↓
POST /install/report
        ↓
后端绑定安装来源
        ↓
注册 / 首充 / 付费继续关联渠道
```

---

## 3. 第一版服务架构

第一版不建议拆成大量微服务。

建议只拆分为：

1. `core-api`
2. `apk-builder`

整体架构：

```text
               Vue Admin
                   |
                   |
               Core API
                   |
       -------------------------
       |           |           |
     MySQL       Redis      S3 / MinIO
                   |
                   |
              Build Queue
                   |
          -------------------
          |        |        |
      Builder1 Builder2 Builder3
          |
    Android SDK / JDK
    Gradle / apksigner
```

### 3.1 Core API

负责：

- 用户
- 租户
- 应用管理
- 渠道管理
- 构建任务
- APK 版本
- 签名配置
- 下载链接
- 安装上报
- 注册归因
- 渠道统计
- 套餐 / 构建额度
- 审计日志

第一版建议作为模块化单体开发。

### 3.2 APK Builder

Builder 必须独立，因为构建 APK：

- CPU 消耗较高
- 内存消耗较高
- 单次构建耗时较长
- 需要 Android SDK
- 需要 JDK
- 需要 Gradle
- 需要签名工具
- 需要临时构建目录
- 后续需要横向扩容

Builder 第一版不一定需要做复杂 HTTP 服务，可以直接作为 Worker：

```text
Redis Queue
    ↓
Builder Worker
    ↓
Gradle Build
    ↓
Sign
    ↓
Upload
    ↓
更新 t_build_task
```

---

## 4. 为什么不建议第一版拆很多微服务

不建议一开始拆成：

```text
user-service
tenant-service
app-service
channel-service
build-service
sign-service
version-service
download-service
analytics-service
billing-service
```

因为第一版会把大量时间浪费在：

- RPC
- 服务发现
- 配置中心
- Trace
- DTO
- 跨服务调用
- 跨服务事务
- 部署
- 日志聚合
- 服务版本兼容

第一版更重要的是快速完成产品闭环。

推荐：

```text
V1
├── core-api
└── apk-builder
```

后期再根据性能和业务边界拆分。

---

## 5. 后期服务拆分建议

### V2

```text
core-api
apk-builder
attribution-service
```

将点击、下载、安装、注册等归因事件独立。

### V3

```text
account-service
app-service
build-service
attribution-service
analytics-service
billing-service
```

当客户量、事件量和构建量明显上升后再拆。

---

## 6. 渠道设计

渠道表建议：

```sql
CREATE TABLE t_promotion_channel (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    tenant_id BIGINT NOT NULL,
    app_id BIGINT NOT NULL,
    channel_code VARCHAR(32) NOT NULL,
    channel_name VARCHAR(100) NOT NULL,
    status TINYINT NOT NULL DEFAULT 1,
    create_time DATETIME NOT NULL,
    update_time DATETIME DEFAULT NULL,

    UNIQUE KEY uk_channel_code (channel_code),
    INDEX idx_tenant_app (tenant_id, app_id)
);
```

每个渠道：

```text
channelCode = 528b128
```

对应推广链接：

```text
https://download.example.com/c/528b128
```

---

## 7. APK 内写入 channelCode

建议不要修改 `packageName` 来区分渠道。

所有渠道保持：

```text
applicationId = com.xxx.app
```

只动态写入：

```text
CHANNEL_CODE = 528b128
```

Gradle 示例：

```groovy
def channelCode = project.findProperty("CHANNEL_CODE") ?: "official"

android {
    defaultConfig {
        buildConfigField "String",
                "CHANNEL_CODE",
                "\"${channelCode}\""
    }
}
```

构建：

```bash
./gradlew assembleRelease -PCHANNEL_CODE=528b128
```

App 读取：

```java
String channelCode = BuildConfig.CHANNEL_CODE;
```

首次启动上报：

```json
{
  "channelCode": "528b128",
  "installId": "xxxx",
  "appVersion": "1.2.3"
}
```

---

## 8. 安装归因设计

用户首次启动：

```http
POST /api/install/report
```

请求：

```json
{
  "channelCode": "528b128",
  "installId": "xxx",
  "appVersion": "1.0.0"
}
```

后端第一次绑定成功后，不允许客户端随意覆盖渠道。

建议记录：

```text
installId
channelCode
appId
tenantId
firstOpenTime
IP
appVersion
deviceModel
registerUserId
registerTime
```

安装记录表：

```sql
CREATE TABLE t_channel_install (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    tenant_id BIGINT NOT NULL,
    app_id BIGINT NOT NULL,
    channel_code VARCHAR(32) NOT NULL,
    install_id VARCHAR(128) NOT NULL,
    app_version VARCHAR(32),
    first_open_time DATETIME NOT NULL,
    register_user_id BIGINT DEFAULT NULL,
    register_time DATETIME DEFAULT NULL,

    UNIQUE KEY uk_install_id (install_id),
    INDEX idx_channel_code (channel_code),
    INDEX idx_app_channel (app_id, channel_code)
);
```

---

## 9. 渠道统计

后台可统计：

```text
渠道
↓
点击
↓
下载
↓
首次启动
↓
注册
↓
首充
↓
付费
```

示例：

```text
渠道         点击     下载     首启     注册     首充

528b128      10000    6200     4500     3000      850
528b129       8000    5000     3700     1900      320
528b130       5000    3200     2100     1100      180
```

后期统计量较大时可以：

```text
事件
 ↓
Kafka
 ↓
Analytics Service
 ↓
ClickHouse
```

第一版仍可先使用 MySQL。

---

## 10. 动态白标能力

后续可以增加：

```text
channelCode
App Name
Logo
启动图
API Host
packageName
Telegram Bot
主题颜色
签名
```

从渠道平台升级为：

> Android White-label Builder

其中 `packageName` 只有真正白标需求才建议修改。

单纯渠道统计不建议修改 packageName。

---

## 11. 签名管理

所有普通渠道包建议：

```text
同一个 applicationId
+
同一个 release keystore
+
不同 channelCode
```

不要：

```text
每个渠道
↓
不同签名
```

否则会导致：

- 无法正常覆盖升级
- 厂商安全信誉难积累
- 推送 / Firebase 配置复杂
- OAuth 配置复杂
- 厂商审核复杂

签名文件和密码不建议明文保存。

推荐：

```text
AWS KMS
AWS Secrets Manager
HashiCorp Vault
```

或者至少：

```text
数据库加密
+
对象存储私有桶
+
严格权限
+
审计日志
```

---

## 12. SaaS 与私有部署

建议一开始支持多租户设计。

核心表增加：

```text
tenant_id
```

SaaS：

```text
一套系统
↓
Tenant A
Tenant B
Tenant C
```

私有部署：

```text
一套系统
↓
单个 Tenant
```

这样业务代码无需维护两套。

产品可以分为：

| 模式 | 适合客户 | 收费方式 |
|---|---|---|
| SaaS | 小团队 / 开发者 | 月付 / 年付 |
| Dedicated | 中型客户 | 独立实例 + 年费 |
| Private Deployment | 企业 | 部署费 + 维护费 |

---

## 13. 权限系统

第一版不需要复杂菜单权限。

建议只做固定角色：

```text
owner
admin
viewer
```

### owner

- 企业全部权限
- 成员管理
- 签名管理
- 套餐
- 应用
- 渠道
- 构建

### admin

- 应用
- 渠道
- 构建
- 版本

### viewer

- 查看应用
- 查看渠道
- 查看统计

第一版不建议做：

```text
菜单表
角色菜单表
按钮权限
接口权限表
动态路由
```

等企业客户真正提出更复杂权限需求后再升级 RBAC。

---

## 14. 第一版后台菜单

建议：

```text
首页 Dashboard

应用管理
├── 应用列表
├── 应用配置
└── 版本管理

渠道管理
├── 渠道列表
├── 推广链接
└── 渠道统计

构建中心
├── 构建任务
├── 构建日志
└── APK 下载

签名管理

安装归因
├── 首次启动
├── 注册归因
└── 转化统计

团队
├── 成员
└── 角色

套餐与用量

系统
├── 操作日志
└── 基础设置
```

---

## 15. 推荐技术栈

### 管理后台

```text
Vue 3
Element Plus
Vite
```

### Core API

```text
Go
go-zero / Kratos
MySQL
Redis
```

### Builder

```text
Go Worker
Docker
JDK
Android SDK
Gradle
apksigner
```

### 队列

第一版：

```text
Redis Streams / Redis Queue
```

后期：

```text
Kafka
```

### 文件存储

```text
AWS S3
MinIO
OSS
```

### CDN

```text
CloudFront
Cloudflare
其他 CDN
```

---

## 16. Builder 横向扩容

Builder 天然适合横向扩容：

```text
             Redis Queue
                  |
      -------------------------
      |          |            |
  Builder-1  Builder-2    Builder-3
```

所有 Builder 消费同一个任务队列。

可以根据构建量扩容：

```text
1 台
↓
3 台
↓
10 台
```

Core API 不需要修改。

---

## 17. 构建任务状态

建议：

```text
PENDING
BUILDING
SIGNING
UPLOADING
SUCCESS
FAILED
```

表：

```text
t_build_task
```

核心字段：

```text
id
tenant_id
app_id
channel_id
channel_code
version_code
version_name
status
builder_id
apk_url
apk_sha256
error_message
start_time
finish_time
create_time
```

---

## 18. 产品演进路线

### V1

```text
应用
渠道
channelCode
自动 Build APK
签名
上传
推广链接
首次启动上报
渠道统计
```

### V2

```text
AppName
Logo
启动图
API Host
```

### V3

```text
动态 packageName
多套签名
白标模板
```

### V4

```text
多 Builder
构建并发
任务优先级
构建缓存
```

### V5

```text
开放 API
Webhook
CI/CD
GitHub / GitLab 集成
```

### V6

```text
SaaS 计费
套餐
构建额度
存储额度
团队
```

### V7

```text
Dedicated
Private Deployment
Local Builder Agent
```

---

## 19. 最终定位

不建议产品只叫：

```text
APK 动态打包平台
```

更适合的产品方向：

```text
Android App Distribution Platform
```

或：

```text
Mobile App Release Platform
```

核心竞争力应围绕：

```text
动态构建
+
渠道分发
+
安装归因
+
签名管理
+
版本管理
+
白标配置
+
私有部署
```

而不是单纯“生成 APK”。

---

## 20. 第一版开发原则

第一版最重要的是快速形成完整闭环：

```text
创建应用
↓
创建渠道
↓
Build
↓
下载
↓
安装
↓
首次打开
↓
上报
↓
后台看到渠道数据
```

只要这个链路完整，就已经具备产品原型价值。

不要过早投入：

```text
复杂 RBAC
大量微服务
复杂审批流
复杂工作流
复杂 BI
复杂计费
```

先把真正能卖的核心能力做出来。
