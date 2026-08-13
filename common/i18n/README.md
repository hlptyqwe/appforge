# 翻译模块 (i18n) 使用指南

## 概述

i18n 模块是 appforge 项目的国际化翻译模块，主要用于翻译错误消息和系统消息，支持多语言（目前支持中文和英文）。

## 快速开始

### 1. 基本使用

```go
import (
	"context"
	"appforge/common/i18n"
)

// 使用默认语言（中文）翻译错误码
msg := i18n.Translate(i18n.ParamError, context.Background())
// 输出: "参数错误"

// 为context设置特定语言后翻译
ctx := i18n.ContextWithLanguage(context.Background(), i18n.EN)
msg := i18n.Translate(i18n.ParamError, ctx)
// 输出: "Parameter error"
```

### 2. 在 helper.ErrResp 中使用

在 `appforge/common/helper/response.go` 中修改使用示例：

```go
import (
	"context"
	"appforge/common/i18n"
	"appforge/proto/common"
)

// 翻译错误码并返回错误响应
ctx := context.Background() // 从请求中获取
msg := i18n.Translate(i18n.ParamError, ctx)
resp := helper.ErrResp(i18n.ParamError, msg)
```

### 3. 自定义消息

```go
// 使用自定义消息而不是翻译的消息
ctx := context.Background()
msg := i18n.TranslateError(i18n.UserNotFound, "用户ID为123的用户不存在", ctx)
// 输出: "用户ID为123的用户不存在"
```

### 4. 全局设置默认语言

```go
// 在应用启动时设置全局默认语言
i18n.SetDefaultLanguage(i18n.EN) // 设置为英文
// 或
i18n.SetDefaultLanguage(i18n.ZH) // 设置为中文
```

## 支持的错误码

### 常用错误码

| 代码 | 常量                       | 英文                    | 中文           |
| ---- | -------------------------- | ----------------------- | -------------- |
| 200  | CodeOK                     | OK                      | 成功           |
| 400  | CodeBadRequest             | Parameter error         | 参数错误       |
| 401  | CodeUnauthorized           | Unauthorized            | 未授权         |
| 403  | CodeForbidden              | Forbidden               | 禁止访问       |
| 404  | CodeNotFound               | Not found               | 资源不存在     |
| 409  | CodeConflict               | Conflict                | 冲突           |
| 500  | CodeInternalError          | Internal server error   | 服务器内部错误 |
| 503  | CodeServiceUnavailable     | Service unavailable     | 服务不可用     |
| 1001 | CodeOperationNotAllowed    | Operation not allowed   | 操作不被允许   |
| 1002 | CodeResourceExists         | Resource already exists | 资源已存在     |
| 1003 | CodeParamError             | Invalid request         | 无效请求       |
| 1004 | CodeDataValidationError    | Data validation error   | 数据验证错误   |
| 1005 | CodeAuthenticationRequired | Authentication required | 需要验证       |
| 1006 | CodeUserNotFound           | User not found          | 用户不存在     |
| 1007 | CodeInvalidCredentials     | Invalid credentials     | 凭证无效       |
| 1008 | CodeTokenExpired           | Token expired           | 令牌已过期     |
| 1009 | CodePermissionDenied       | Permission denied       | 权限被拒绝     |

## 添加新的错误码

### 1. 在 `messages.go` 中添加翻译

```go
// 添加到 MessageMap
const NewErrorCode = 2001

MessageMap[NewErrorCode] = map[Language]string{
	EN: "Custom error message",
	ZH: "自定义错误消息",
}
```

### 2. 在 `errors.go` 中添加常量

```go
const (
	CodeNewError = 2001
)
```

### 3. 在代码中使用

```go
msg := i18n.Translate(i18n.CodeNewError, ctx)
```

## 从context中获取语言

### 在HTTP中间件中设置

```go
import (
	"net/http"
	"appforge/common/i18n"
)

func LanguageMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lang := r.Header.Get("Accept-Language")
		ctx := r.Context()

		if lang == "en" {
			ctx = i18n.ContextWithLanguage(ctx, i18n.EN)
		} else {
			ctx = i18n.ContextWithLanguage(ctx, i18n.ZH)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
```

### 在gRPC中使用

```go
import (
	"context"
	"appforge/common/i18n"
)

func (l *MyLogic) MyMethod(ctx context.Context, in *Request) (*Response, error) {
	// 从context中获取语言并进行翻译
	lang := i18n.ContextWithLanguage(ctx, i18n.EN)
	msg := i18n.Translate(i18n.ParamError, lang)

	// 返回翻译后的消息
	return &Response{
		Base: helper.ErrResp(i18n.ParamError, msg),
	}, nil
}
```

## API 文档

### 函数

#### `Translate(code int32, ctx context.Context) string`

翻译错误码为消息字符串。根据context中的语言设置进行翻译，若未设置则使用默认语言。

#### `TranslateError(code int32, customMsg string, ctx context.Context) string`

翻译错误，如果提供了customMsg则使用customMsg，否则从翻译字典中查找。

#### `ContextWithLanguage(ctx context.Context, lang Language) context.Context`

为context添加语言设置。

#### `SetDefaultLanguage(lang Language)`

设置全局默认语言。

### 类型

#### `Language` - 语言类型

- `EN` - 英文
- `ZH` - 中文（默认）

#### `ErrorInfo` - 错误信息对象

包含错误码和自定义消息。

## 最佳实践

1. **在API层使用翻译** - 在handler/logic层翻译消息后返回给客户端
2. **保持一致的错误码** - 使用错误码常量而不是硬编码数字
3. **支持客户端语言选择** - 通过Accept-Language头或query参数让客户端选择语言
4. **为特定业务错误添加自定义消息** - 使用TranslateError函数

## 示例

### 完整示例

```go
package logic

import (
	"context"
	"appforge/common/helper"
	"appforge/common/i18n"
	"appforge/proto/user"
)

type LoginLogic struct {
	ctx context.Context
}

func (l *LoginLogic) Login(in *user.LoginReq) (*user.LoginResp, error) {
	// 验证用户
	if in.Username == "" {
		code := i18n.CodeParamError
		msg := i18n.Translate(code, l.ctx)
		return &user.LoginResp{
			Base: helper.ErrResp(int32(code), msg),
		}, nil
	}

	// 检查用户是否存在
	user, err := findUser(in.Username)
	if err != nil {
		code := i18n.CodeUserNotFound
		msg := i18n.TranslateError(code, "用户不存在", l.ctx)
		return &user.LoginResp{
			Base: helper.ErrResp(int32(code), msg),
		}, nil
	}

	// 验证密码
	if !verifyPassword(user, in.Password) {
		code := i18n.CodeInvalidCredentials
		msg := i18n.Translate(code, l.ctx)
		return &user.LoginResp{
			Base: helper.ErrResp(int32(code), msg),
		}, nil
	}

	return &user.LoginResp{
		Base: helper.OkResp(),
		User: convertToProto(user),
	}, nil
}
```

## 注意事项

- 翻译模块是线程安全的
- 默认语言为中文（ZH）
- 如果某语言的翻译不存在，会自动降级到英文

## 新增功能

### 响应构建器 (ResponseBuilder)

为了简化错误响应的构建，提供了响应构建器工具：

```go
// 创建构建器
ctx := context.Background()
builder := i18n.NewResponseBuilder(ctx)

// 构建各种错误响应
builder.ValidationError("错误信息")      // 参数验证错误
builder.NotFoundError("不存在")         // 资源不存在
builder.UnauthorizedError("未授权")     // 未授权
builder.PermissionDeniedError("无权限")  // 权限拒绝
builder.ConflictError("已存在")         // 资源冲突
builder.InternalError("内部错误")       // 服务器错误
```

或使用便利函数：

```go
i18n.BuildValidationErrorResponse(ctx, "错误")
i18n.BuildNotFoundErrorResponse(ctx, "不存在")
i18n.BuildPermissionDeniedErrorResponse(ctx, "无权限")
// ... 更多便利函数
```

### HTTP 中间件

自动从HTTP请求中提取语言并设置到context中：

```go
import "appforge/common/i18n"

// 在HTTP服务器中使用
mux := http.NewServeMux()
handler := i18n.HTTPMiddleware(mux)
http.ListenAndServe(":8080", handler)
```

语言检测优先级：

1. URL query参数 `?lang=en`
2. Accept-Language HTTP头
3. 系统默认语言

### gRPC 拦截器

自动在gRPC服务器中支持语言设置：

```go
import "appforge/common/i18n"

// 创建gRPC服务器
s := grpc.NewServer(
    grpc.UnaryInterceptor(i18n.GRPCUnaryServerInterceptor()),
    grpc.StreamInterceptor(i18n.GRPCStreamServerInterceptor()),
)

// 服务器会自动从metadata中提取语言
// 客户端集成
conn, _ := grpc.Dial(
    "localhost:50051",
    grpc.WithUnaryInterceptor(i18n.GRPCClientUnaryInterceptor()),
    grpc.WithStreamInterceptor(i18n.GRPCClientStreamInterceptor()),
)
```

gRPC语言传播：

- 服务器从 `x-language` 或 `accept-language` metadata中读取
- 客户端如果context中有语言设置，自动在metadata中传递

### 集成指南

详见 [INTEGRATION_GUIDE.md](./INTEGRATION_GUIDE.md)，包含：

- 逐步集成步骤
- 在不同代码层的使用示例
- 中间件配置指南
- 错误处理最佳实践
- 添加自定义错误码的方法
- 常见问题解答
- Context中的语言设置优先级高于全局默认语言
