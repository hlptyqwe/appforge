# i18n 模块项目文档索引

> 国际化翻译模块完整文档和资源导航

## 📚 文档地图

### 🎯 入门 (新用户必看)

#### 1️⃣ 最快开始 (2分钟)

**文件**: [QUICK_REFERENCE.md](./QUICK_REFERENCE.md)

- 一页纸速查指南
- 秒速开始（3步）
- 常用代码片段
- 错误码对照表

#### 2️⃣ 快速开始 (10分钟)

**文件**: [README.md](./README.md)

- 概述和特性
- 4步基本使用
- 18个错误码参考
- API 文档
- 最佳实践

#### 3️⃣ 完整集成 (30分钟)

**文件**: [INTEGRATION_GUIDE.md](./INTEGRATION_GUIDE.md)

- 逐步集成步骤
- 在不同代码层的使用
- 中间件完整配置
- 5种错误处理模式
- 添加自定义错误码
- 10个常见问题解答

### 📖 详细文档

#### 架构和设计

**文件**: [MODULES_OVERVIEW.md](./MODULES_OVERVIEW.md)

- 文件结构和说明
- 每个模块的详细介绍
- 工作流程和流程图
- 文件依赖关系
- 功能总结和矩阵

#### 项目改进

**文件**: [IMPROVEMENTS_SUMMARY.md](./IMPROVEMENTS_SUMMARY.md)

- 两个阶段的改进总结
- 分页统一 (35个方法)
- i18n模块创建详情
- 改进指标和成果
- 集成清单
- 未来规划

#### 版本历史

**文件**: [CHANGELOG.md](./CHANGELOG.md)

- v1.0.0 初始发布详情
- 功能清单
- 代码统计
- 质量指标
- 发布检查清单
- 未来版本计划

### 💻 源代码

#### 核心模块

##### 消息字典

**文件**: `messages.go` (~100 行)

- Language 类型定义
- 18个错误码常量
- MessageMap 全局字典
- 所有翻译

##### 翻译引擎

**文件**: `translator.go` (~150 行)

- Translator 结构体
- 全局翻译器实例
- Translate() 函数
- Context 语言管理
- 3级降级机制

##### 错误处理

**文件**: `errors.go` (~80 行)

- ErrorInfo 结构体
- 13个错误码常量
- TranslateError() 函数
- 错误信息管理

##### 快速工具

**文件**: `response_builder.go` (~150 行)

- ResponseBuilder 类
- 8个快速错误响应
- 便利函数集合
- Context 感知响应

#### 中间件和集成

**文件**: `middleware.go` (~250 行)

- HTTPMiddleware 中间件
- Accept-Language 解析
- GRPCUnaryServerInterceptor
- GRPCStreamServerInterceptor
- GRPCClientUnaryInterceptor
- GRPCClientStreamInterceptor
- 完整的语言传播

#### 示例代码

**文件**: `examples.go` (~400 行)

- 10个详细示例
- Logic 层使用
- ResponseBuilder 使用
- 自定义错误码
- Handler 层使用
- 中间件配置
- 权限处理
- 完整业务逻辑

### 🧪 测试

#### 翻译器测试

**文件**: `translator_test.go` (~300 行)

- 9个单元测试
- 翻译功能测试
- Context 语言测试
- 降级机制测试
- 性能基准测试

#### 中间件测试

**文件**: `middleware_test.go` (~400 行)

- HTTP 中间件测试
- Accept-Language 解析
- ResponseBuilder 测试
- gRPC 集成测试
- 并发访问测试
- 4个性能基准测试

## 🎓 学习路径

### 路径 1: 5分钟快速了解

1. [QUICK_REFERENCE.md](./QUICK_REFERENCE.md) - 快速参考
2. 看看本项目结构

### 路径 2: 30分钟基础掌握

1. [QUICK_REFERENCE.md](./QUICK_REFERENCE.md) - 快速参考 (5分钟)
2. [README.md](./README.md) - 快速开始 (10分钟)
3. 浏览 `examples.go` 代码示例 (10分钟)
4. 尝试在项目中使用 (5分钟)

### 路径 3: 1小时完全理解

1. [QUICK_REFERENCE.md](./QUICK_REFERENCE.md) (5分钟)
2. [README.md](./README.md) (10分钟)
3. [INTEGRATION_GUIDE.md](./INTEGRATION_GUIDE.md) (20分钟)
4. [MODULES_OVERVIEW.md](./MODULES_OVERVIEW.md) (15分钟)
5. 阅读源代码 (10分钟)

### 路径 4: 深度学习

1. 完整阅读所有文档 (1小时)
2. 阅读所有源代码 (1小时)
3. 运行所有单元测试 (30分钟)
4. 编写自己的测试 (30分钟)

## 🔍 按功能查找

### 我想...

#### 快速开始

- ➜ [QUICK_REFERENCE.md](./QUICK_REFERENCE.md) - 秒速开始
- ➜ [README.md](./README.md) - 基础使用

#### 在我的项目中集成

- ➜ [INTEGRATION_GUIDE.md](./INTEGRATION_GUIDE.md) - Step by step

#### 理解架构

- ➜ [MODULES_OVERVIEW.md](./MODULES_OVERVIEW.md) - 完整解析

#### 看代码示例

- ➜ `examples.go` - 10个实际示例
- ➜ `translator_test.go` - 单元测试示例
- ➜ `middleware_test.go` - 中间件示例

#### 添加自定义错误码

- ➜ [QUICK_REFERENCE.md](./QUICK_REFERENCE.md#⚙️-添加自定义错误码) - 3步指南
- ➜ [INTEGRATION_GUIDE.md](./INTEGRATION_GUIDE.md#添加自定义错误码) - 详细说明

#### 配置中间件

- ➜ [INTEGRATION_GUIDE.md](./INTEGRATION_GUIDE.md#中间件配置) - 完整配置

#### 解决问题

- ➜ [INTEGRATION_GUIDE.md](./INTEGRATION_GUIDE.md#常见问题) - FAQ
- ➜ [QUICK_REFERENCE.md](./QUICK_REFERENCE.md#📞-快速帮助) - 快速助手

#### 了解改进历史

- ➜ [IMPROVEMENTS_SUMMARY.md](./IMPROVEMENTS_SUMMARY.md) - 项目改进总结
- ➜ [CHANGELOG.md](./CHANGELOG.md) - 版本历史

## 📊 文件统计

### 源代码文件

| 文件                | 行数 | 用途     |
| ------------------- | ---- | -------- |
| messages.go         | ~100 | 消息字典 |
| translator.go       | ~150 | 翻译引擎 |
| errors.go           | ~80  | 错误工具 |
| response_builder.go | ~150 | 响应构建 |
| middleware.go       | ~250 | 中间件   |
| examples.go         | ~400 | 使用示例 |

**源代码总计**: ~1130 行

### 测试文件

| 文件               | 行数 | 用途       |
| ------------------ | ---- | ---------- |
| translator_test.go | ~300 | 翻译器测试 |
| middleware_test.go | ~400 | 中间件测试 |

**测试代码总计**: ~700 行

### 文档文件

| 文件                    | 行数 | 用途     |
| ----------------------- | ---- | -------- |
| README.md               | ~200 | 快速开始 |
| QUICK_REFERENCE.md      | ~300 | 速查卡   |
| INTEGRATION_GUIDE.md    | ~500 | 集成指南 |
| MODULES_OVERVIEW.md     | ~400 | 模块概览 |
| IMPROVEMENTS_SUMMARY.md | ~400 | 改进总结 |
| CHANGELOG.md            | ~300 | 版本历史 |

**文档总计**: ~2100 行

**总计**: ~3930 行代码和文档

## 🎯 快速导航

### 按用户角色

#### 👤 新用户

1. [QUICK_REFERENCE.md](./QUICK_REFERENCE.md) - 速查卡
2. [README.md](./README.md) - 基础

#### 👨‍💼 项目经理

1. [IMPROVEMENTS_SUMMARY.md](./IMPROVEMENTS_SUMMARY.md) - 改进总结
2. [CHANGELOG.md](./CHANGELOG.md) - 版本历史

#### 👨‍💻 开发者

1. [QUICK_REFERENCE.md](./QUICK_REFERENCE.md) - 快速参考
2. [INTEGRATION_GUIDE.md](./INTEGRATION_GUIDE.md) - 集成指南
3. `examples.go` - 代码示例

#### 🏗️ 架构师

1. [MODULES_OVERVIEW.md](./MODULES_OVERVIEW.md) - 架构设计
2. 源代码各文件 - 实现细节

#### 🧪 测试工程师

1. `translator_test.go` - 翻译器测试
2. `middleware_test.go` - 中间件测试

### 按任务类型

#### 任务: 快速评估项目

- 时间: 5分钟
- 资源: [QUICK_REFERENCE.md](./QUICK_REFERENCE.md)

#### 任务: 学习基本使用

- 时间: 30分钟
- 资源: [README.md](./README.md) + [QUICK_REFERENCE.md](./QUICK_REFERENCE.md)

#### 任务: 完整集成到项目

- 时间: 1-2小时
- 资源: [INTEGRATION_GUIDE.md](./INTEGRATION_GUIDE.md) + `examples.go`

#### 任务: 理解内部架构

- 时间: 2小时
- 资源: [MODULES_OVERVIEW.md](./MODULES_OVERVIEW.md) + 源代码

#### 任务: 扩展或改进模块

- 时间: 3小时+
- 资源: 所有文档 + 所有源代码 + 测试

## 🔗 相关项目

### appforge 微服务平台

#### 相关模块

- `appforge/common/pageutil` - 分页工具
- `appforge/common/helper` - 辅助函数
- `appforge/proto/common` - 通用Protocol Buffers

#### 相关服务

- Payment 服务
- Asset 服务
- Market 服务
- User 服务
- System 服务

## 📞 获取帮助

### 遇到问题

1. 查看 → [INTEGRATION_GUIDE.md](./INTEGRATION_GUIDE.md#常见问题)
2. 查看 → [QUICK_REFERENCE.md](./QUICK_REFERENCE.md#📞-快速帮助)
3. 查看 → `examples.go` 中的示例

### 提交建议

- 添加错误码
- 添加新语言
- 改进文档
- 性能优化

### 报告问题

- API 问题
- 集成问题
- 性能问题
- 文档问题

## 📋 检查清单

### 使用前

- [ ] 已阅读 [QUICK_REFERENCE.md](./QUICK_REFERENCE.md)
- [ ] 已理解最小化示例
- [ ] 已查看 `examples.go`

### 集成时

- [ ] 按照 [INTEGRATION_GUIDE.md](./INTEGRATION_GUIDE.md) 步骤
- [ ] 已配置中间件
- [ ] 已设置默认语言
- [ ] 已运行单元测试

### 投入生产

- [ ] 已通过单元测试
- [ ] 已验证多语言
- [ ] 已性能测试
- [ ] 已更新文档

## 🚀 快捷操作

### 复制粘贴代码

#### 最小集成

```go
// 1. main.go
import "appforge/common/i18n"
func init() { i18n.SetDefaultLanguage(i18n.ZH) }

// 2. Logic
return i18n.BuildValidationErrorResponse(ctx, "")
```

#### HTTP中间件

```go
handler := i18n.HTTPMiddleware(mux)
http.ListenAndServe(":8080", handler)
```

#### gRPC拦截器

```go
grpc.NewServer(
    grpc.UnaryInterceptor(i18n.GRPCUnaryServerInterceptor()),
)
```

### 查询常用

#### 翻译错误码

[查看 QUICK_REFERENCE.md](./QUICK_REFERENCE.md#🔢-常用错误码)

#### 添加错误码

[查看 QUICK_REFERENCE.md](./QUICK_REFERENCE.md#⚙️-添加自定义错误码)

#### 处理多语言

[查看 QUICK_REFERENCE.md](./QUICK_REFERENCE.md#📝-在-logic-中使用)

## 📝 文档版本

- **主版本**: 1.0.0
- **发布日期**: 2024年
- **最后更新**: 2024年
- **状态**: ✅ 完整
- **维护者**: Development Team

---

**导航提示**:

- 新用户 → 从 [QUICK_REFERENCE.md](./QUICK_REFERENCE.md) 开始
- 开发者 → 查阅 [INTEGRATION_GUIDE.md](./INTEGRATION_GUIDE.md)
- 架构师 → 阅读 [MODULES_OVERVIEW.md](./MODULES_OVERVIEW.md)

**文档最后更新**: 2024年
