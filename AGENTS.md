# appforge 项目开发规则

## 项目结构

- `services/system` 负责系统基础能力，包括用户、租户、角色、菜单权限和系统配置。
- `services/core` 负责 APK 渠道动态打包平台的核心业务和业务数据。
- `services/builder` 负责构建任务编排、构建执行和构建结果处理。
- `builder` 不重复定义 `core` 的业务表；跨服务业务数据通过 RPC 访问所属服务。
- 保留 `system` 服务中的菜单权限相关能力，不因简化角色模型而删除菜单权限表。

## 数据库规则

- 系统表使用 `sys_` 前缀。
- 业务表必须使用 `t_` 前缀。
- 每张表、每个字段、每个索引和枚举值都必须有清晰的中文注释。
- 修改表结构时同步检查 SQL、模型、RPC 定义和相关业务代码。
- 不要在多个服务中重复创建同一业务表；先明确表的归属服务。
- 删除表或字段前必须确认没有代码、模型、查询和 RPC 依赖。

## Go、RPC 和模型

- 使用 Go 编写服务代码，修改 Go 文件后运行 `gofmt`。
- 状态、类型、平台、来源等固定取值必须定义为 enum，并添加中文注释；不要使用无注释的裸数字或字符串代替。
- Protobuf 的 service、message、field、enum 和 enum value 都要添加中文注释。
- 每个服务的 Makefile 必须提供 `gen-model` 目标；没有模型生成任务的服务也要保留该目标并明确说明。
- 修改数据库或 Protobuf 后，执行对应服务的模型生成和代码生成命令。
- 优先通过已有 RPC 接口复用其他服务能力，不直接访问其他服务负责的业务表。

## Make 命令使用规则

所有命令必须在对应 Makefile 所在目录执行，不要在仓库根目录猜测执行目标。

### Proto 代码生成

- 修改 `proto/common/*.proto` 后，在 `proto/common` 执行 `make gen`。
- 修改 `proto/system/*.proto` 后，在 `proto/system` 执行 `make gen`。
- 修改 `proto/core/core.proto` 后，在 `proto/core` 执行 `make gen`。
- 修改 `proto/builder/builder.proto` 后，在 `proto/builder` 执行 `make gen`。
- `proto/*/make gen` 用于把 Protobuf 定义生成 Go、gRPC 代码；只有修改 `.proto` 或需要重新生成 PB 代码时执行。

### Service RPC 代码生成

- 修改 RPC 的 service、message、field 或 enum 定义后，先生成对应的 `proto` 代码，再在对应服务目录执行 `make gen`。
- `services/system`、`services/core`、`services/builder` 中的 `make gen` 用于通过 goctl 生成 RPC server、handler、types 和相关基础代码，同时会执行必要的格式化和依赖整理。
- 只有修改服务 RPC 契约，或明确需要重建 goctl 生成代码时，才执行 services 的 `make gen`。
- 只修改业务 logic、配置或普通 Go 代码时，不执行 proto `make gen` 和 services `make gen`。

### 数据库模型生成

- 修改 `services/system/system.sql` 后，在 `services/system` 执行 `make gen-model`。
- 修改 `services/core/core.sql` 后，在 `services/core` 执行 `make gen-model`。
- `services/builder` 当前不拥有业务表；其 `make gen-model` 只用于明确说明模型由 `services/core` 维护，不生成模型。
- `make gen-model` 只在新增、修改或删除 SQL 表/字段/索引后执行；不要因为普通 Go 代码修改而执行。

### API 代码生成和格式化

- 修改 `appforge-api/api/*.api` 后，在 `appforge-api` 先执行 `make fmt-api`，再执行 `make gen`。
- `appforge-api/make gen` 用于根据 API 定义生成 go-zero API 的 types、handler、routes 等代码。
- 修改或生成 Go 代码后，在 `appforge-api` 执行 `make fmt`；`make fmt` 只负责格式化 Go 文件。
- 只修改 Go 业务逻辑时执行 `make fmt`，不执行 `make fmt-api` 或 API `make gen`。

### 跨层变更顺序

- 同时修改 Protobuf 和 RPC 服务时，按以下顺序执行：`proto/*/make gen`，再执行对应 `services/*/make gen`。
- 同时修改 SQL 和模型使用代码时，先执行对应 `services/*/make gen-model`，再检查并格式化 Go 代码。
- 修改 API、RPC 和 Go 代码时，按定义层到实现层执行：API `make fmt-api` → API `make gen` → proto `make gen` → services `make gen` → 相关目录 `make fmt`。
- 生成完成后运行受影响服务的测试；生成文件不得手工修改，必须修改源文件后重新执行对应的生成命令。

## 修改和验证

- 只修改完成当前任务所需的文件，不覆盖或删除无关的用户改动。
- 修改数据库后检查表名、字段注释、索引、唯一约束、外键关系和初始化数据。
- 修改后运行受影响服务的测试；条件允许时运行 `go test ./...`。
- 生成文件应通过项目已有的生成命令更新，不手工修改生成结果。
