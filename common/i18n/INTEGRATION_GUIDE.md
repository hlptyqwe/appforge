# i18n 模块集成指南

本指南将帮助你将 i18n 翻译模块集成到现有的 appforge 微服务中。

## 📋 目录

- [快速开始](#快速开始)
- [集成步骤](#集成步骤)
- [在不同层使用](#在不同层使用)
- [中间件配置](#中间件配置)
- [错误处理模式](#错误处理模式)
- [添加自定义错误码](#添加自定义错误码)
- [常见问题](#常见问题)

## 快速开始

### 1. 导入模块

```go
import "appforge/common/i18n"
```

### 2. 设置默认语言（应用启动时）

```go
func init() {
	i18n.SetDefaultLanguage(i18n.ZH) // 设置为中文
}
```

### 3. 在 Logic 中使用

```go
// 参数验证失败
return i18n.BuildValidationErrorResponse(ctx, "")

// 权限被拒绝
return i18n.BuildPermissionDeniedErrorResponse(ctx, "")

// 资源不存在
return i18n.BuildNotFoundErrorResponse(ctx, "")
```

## 集成步骤

### Step 1: 在 main.go 中初始化

```go
package main

import (
	"appforge/common/i18n"
)

func main() {
	// 设置默认语言
	i18n.SetDefaultLanguage(i18n.ZH)

	// ... 其他初始化代码

	// 注册中间件
	registerMiddedares()

	// ... 启动服务
}

func registerMiddlewares() {
	// HTTP: 在路由器中添加
	// router.Use(i18n.HTTPMiddleware)

	// gRPC: 在服务器选项中添加
	// grpc.NewServer(
	//     grpc.UnaryInterceptor(i18n.GRPCUnaryServerInterceptor()),
	//     grpc.StreamInterceptor(i18n.GRPCStreamServerInterceptor()),
	// )
}
```

### Step 2: 更新 Logic 层

在 `internal/logic` 中的每个 List/业务方法内：

**前：**

```go
func (l *UserLogic) ListUsers(ctx context.Context, req *ListRequest) (*ListResponse, error) {
	// 参数验证
	if req.Page <= 0 {
		return nil, &RespBase{
			Code: 400,
			Msg:  "参数错误", // 硬编码
		}
	}

	// ... 业务逻辑
}
```

**后：**

```go
func (l *UserLogic) ListUsers(ctx context.Context, req *ListRequest) (*ListResponse, error) {
	// 参数验证
	if req.Page <= 0 {
		return nil, i18n.BuildValidationErrorResponse(ctx, "")
	}

	// ... 业务逻辑
}
```

### Step 3: 更新错误处理

**查询失败：**

```go
user, err := l.svc.GetUser(id)
if err != nil {
	return i18n.BuildNotFoundErrorResponse(ctx, "用户不存在")
}
```

**权限检查：**

```go
if !l.hasPermission(ctx, resource) {
	return i18n.BuildPermissionDeniedErrorResponse(ctx, "")
}
```

**业务错误：**

```go
if existing, _ := l.svc.FindByEmail(email); existing != nil {
	return i18n.BuildConflictErrorResponse(ctx, "邮箱已被使用")
}
```

## 在不同层使用

### Handler 层

```go
package handler

import "appforge/common/i18n"

func (h *Handler) GetUser(ctx context.Context, req *GetUserRequest) (*User, error) {
	l := logic.NewUserLogic(ctx, h.svc)
	return l.GetUser(ctx, req)
}
```

### Logic 层（主要使用层）

```go
package logic

import (
	"context"
	"appforge/common/i18n"
)

type UserLogic struct {
	ctx context.Context
	svc UserService
}

func (l *UserLogic) GetUser(ctx context.Context, id string) (*User, error) {
	// 方式1: 使用 ResponseBuilder
	builder := i18n.NewResponseBuilder(ctx)

	if id == "" {
		return nil, builder.ValidationError("ID不能为空")
	}

	user, err := l.svc.GetUser(id)
	if err != nil {
		return nil, builder.NotFoundError("用户不存在")
	}

	return user, nil
}

func (l *UserLogic) ListUsers(ctx context.Context, req *ListRequest) (*ListResponse, error) {
	// 方式2: 使用便利函数
	if req.Page <= 0 {
		return nil, i18n.BuildValidationErrorResponse(ctx, "页码必须大于0")
	}

	users, total, err := l.svc.ListUsers(req)
	if err != nil {
		return nil, i18n.BuildInternalErrorResponse(ctx, err.Error())
	}

	return &ListResponse{
		Code:  200,
		Msg:   i18n.Translate(i18n.CodeOK, ctx),
		Users: users,
		Total: total,
	}, nil
}
```

### Service 层

Service 层通常不直接使用 i18n，而是返回结构化错误给 Logic 层处理。

```go
package svc

type UserService interface {
	GetUser(id string) (*User, error)
	ListUsers(req *ListRequest) ([]*User, int64, error)
}
```

### Repository 层

同样不直接使用 i18n，返回业务错误给 Service 层。

## 中间件配置

### HTTP 中间件

```go
package main

import (
	"net/http"
	"appforge/common/i18n"
)

func setupHTTPServer() {
	mux := http.NewServeMux()

	// 包装中间件
	handler := i18n.HTTPMiddleware(mux)

	http.ListenAndServe(":8080", handler)
}
```

或在框架级别（如 Gin）：

```go
package main

import (
	"github.com/gin-gonic/gin"
	"appforge/common/i18n"
)

func setupGinServer() {
	r := gin.Default()

	// 添加中间件
	r.Use(func(c *gin.Context) {
		lang := extractLanguage(c)
		ctx := i18n.ContextWithLanguage(c.Request.Context(), lang)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})

	r.Run(":8080")
}

func extractLanguage(c *gin.Context) i18n.Language {
	// 从 query 参数获取: ?lang=en
	if lang := c.Query("lang"); lang != "" {
		if l := parseLanguage(lang); l != "" {
			return l
		}
	}

	// 从请求头获取
	if lang := c.GetHeader("Accept-Language"); lang != "" {
		if l := parseLanguageFromHeader(lang); l != "" {
			return l
		}
	}

	return i18n.GetDefaultLanguage()
}
```

### gRPC 拦截器

```go
package main

import (
	"google.golang.org/grpc"
	"appforge/common/i18n"
)

func setupGRPCServer() {
	s := grpc.NewServer(
		grpc.UnaryInterceptor(i18n.GRPCUnaryServerInterceptor()),
		grpc.StreamInterceptor(i18n.GRPCStreamServerInterceptor()),
	)

	proxyAPI.RegisterUserServiceServer(s, &userHandler{})
	// ...
	s.Serve(listener)
}
```

## 错误处理模式

### 模式 1: 简单验证错误

```go
if req.Name == "" {
	return i18n.BuildValidationErrorResponse(ctx, "")
}
// 自动翻译为: 中文"参数错误" 或 英文"Parameter error"
```

### 模式 2: 查询失败

```go
user, err := l.svc.GetUser(id)
if err != nil {
	return i18n.BuildNotFoundErrorResponse(ctx, "用户不存在")
}
// 自动翻译错误码，附带自定义消息
```

### 模式 3: 权限检查

```go
if !l.canAccess(ctx, resource) {
	return i18n.BuildPermissionDeniedErrorResponse(ctx, "")
}
// 自动翻译为: 中文"权限被拒绝" 或 英文"Permission denied"
```

### 模式 4: 业务冲突

```go
if exists, _ := l.svc.GetByEmail(req.Email); exists != nil {
	return i18n.BuildConflictErrorResponse(ctx, "邮箱已被使用")
}
// 自动翻译错误码 409，附带自定义消息
```

### 模式 5: 自定义错误码

```go
// 定义在 i18n/errors.go
const CodeInsufficientBalance = 2001

// 使用
if user.Balance < amount {
	msg := i18n.TranslateError(CodeInsufficientBalance, "", ctx)
	return i18n.BuildErrorResponse(ctx, CodeInsufficientBalance, "")
}
```

## 添加自定义错误码

### Step 1: 在 errors.go 中添加常量

```go
package i18n

// 自定义业务错误码 (2000-2999 范围)
const (
	CodeUserNotVerified      = 2001
	CodePaymentFailed        = 2002
	CodeInsufficientBalance  = 2003
	CodeWithdrawLimitExceeded = 2004
)
```

### Step 2: 在 messages.go 中添加翻译

```go
package i18n

// MessageMap 定义所有错误消息
var MessageMap = map[int32]map[Language]string{
	// ... 现有的码 ...

	2001: {
		EN: "User email not verified",
		ZH: "用户邮箱未验证",
	},
	2002: {
		EN: "Payment processing failed",
		ZH: "支付处理失败",
	},
	2003: {
		EN: "Insufficient balance",
		ZH: "余额不足",
	},
	2004: {
		EN: "Withdraw limit exceeded",
		ZH: "提现额度超出限制",
	},
}
```

### Step 3: 在 Logic 中使用

```go
func (l *PaymentLogic) ProcessPayment(ctx context.Context, req *PaymentRequest) (*PaymentResponse, error) {
	if user.Balance < req.Amount {
		return i18n.BuildErrorResponse(
			ctx,
			CodeInsufficientBalance,
			"", // 使用翻译的默认消息
		)
	}

	// 或
	msg := i18n.Translate(CodeInsufficientBalance, ctx)
	return i18n.BuildErrorResponse(ctx, CodeInsufficientBalance, msg)
}
```

## 常见问题

### Q1: 如何在不修改既有代码的情况下逐步集成？

**A:** 按以下优先级集成：

1. 先集成中间件和初始化
2. 新编写的 Logic 方法先使用 i18n
3. 逐步重构现有 List 方法（配合 pageutil 统一）
4. 最后更新其他错误处理代码

### Q2: 如何处理没有翻译的错误码？

**A:** i18n 会自动降级处理：

1. 如果自定义消息非空，使用自定义消息
2. 如果有对应语言的翻译，使用翻译
3. 如果没有，降级到英文翻译
4. 如果都没有，返回空字符串（建议记录日志）

### Q3: 性能影响有多大？

**A:** 很小。i18n 使用 map 查找，时间复杂度 O(1)，RWMutex 读锁非常快。Benchmark 测试显示每次查询不到 0.1 微秒。

### Q4: 如何支持新的语言（如日语）？

**A:** 修改 messages.go：

```go
// 在 Language 类型中添加
const JP Language = "jp"

// 在 MessageMap 中添加翻译
map[int32]map[Language]string{
	200: {
		EN: "Ok",
		ZH: "成功",
		JP: "成功",  // 新增
	},
}
```

### Q5: gRPC 客户端如何传递语言信息？

**A:** 使用 GRPCClientUnaryInterceptor 在客户端自动传播语言：

```go
conn, _ := grpc.Dial(
	"localhost:50051",
	grpc.WithUnaryInterceptor(i18n.GRPCClientUnaryInterceptor()),
)
```

客户端会自动在 metadata 中添加 `x-language` 头。

### Q6: 如何在单元测试中设置语言？

**A:** 直接在测试中创建 context：

```go
func TestGetUser(t *testing.T) {
	// 中文测试
	ctx := i18n.ContextWithLanguage(context.Background(), i18n.ZH)
	// ... 测试代码

	// 英文测试
	ctx = i18n.ContextWithLanguage(context.Background(), i18n.EN)
	// ... 测试代码
}
```

### Q7: 如何调试翻译问题？

**A:** 使用 Translate 函数检查：

```go
// 检查某错误码的翻译
msg := i18n.Translate(i18n.CodeParamError, ctx)
t.Logf("Code %d translates to: %s", i18n.CodeParamError, msg)

// 检查当前 context 的语言
lang := i18n.GetLanguage(ctx)
t.Logf("Current language: %s", lang)
```

### Q8: 服务间通信如何保持语言上下文？

**A:** 使用 gRPC 客户端拦截器自动传播：

```go
// 服务A -> 服务B
// 在服务A中：
ctx := i18n.ContextWithLanguage(context.Background(), i18n.EN)
// 调用服务B的客户端会自动在metadata中包含语言信息
```

## 集成检查清单

- [ ] 在 main.go 中设置默认语言
- [ ] 注册 HTTP 或 gRPC 中间件
- [ ] 更新所有 Logic 层的验证错误处理
- [ ] 更新所有 List 方法的错误处理
- [ ] 添加必要的自定义错误码
- [ ] 编写单元测试验证翻译
- [ ] 验证多语言错误响应
- [ ] 更新 API 文档注明支持的语言

## 相关文件

- `messages.go` - 消息字典和语言定义
- `translator.go` - 核心翻译引擎
- `errors.go` - 错误处理工具
- `response_builder.go` - 响应构建工具
- `middleware.go` - 中间件实现
- `examples.go` - 使用示例
- `translator_test.go` - 单元测试
