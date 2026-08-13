# i18n 快速参考卡

> 一页纸速查指南

## 🚀 秒速开始

### 1. 初始化（main.go）

```go
import "appforge/common/i18n"

func init() {
	i18n.SetDefaultLanguage(i18n.ZH)
}
```

### 2. 注册中间件

```go
// HTTP
mux := http.NewServeMux()
handler := i18n.HTTPMiddleware(mux)

// 或 gRPC
grpc.NewServer(
	grpc.UnaryInterceptor(i18n.GRPCUnaryServerInterceptor()),
)
```

### 3. 在 Logic 中使用

```go
// 验证错误
if req.Name == "" {
	return i18n.BuildValidationErrorResponse(ctx, "")
}

// 不存在
return i18n.BuildNotFoundErrorResponse(ctx, "")

// 权限
return i18n.BuildPermissionDeniedErrorResponse(ctx, "")

// 自定义错误
msg := i18n.Translate(code, ctx)
return helper.ErrResp(code, msg)
```

## 📦 核心 API

### 翻译函数

```go
// 翻译错误码
msg := i18n.Translate(i18n.CodeParamError, ctx)

// 翻译 + 自定义消息
msg := i18n.TranslateError(code, "自定义", ctx)

// 获取当前语言
lang := i18n.GetLanguage(ctx)

// 设置context语言
ctx := i18n.ContextWithLanguage(ctx, i18n.EN)

// 设置全局默认语言
i18n.SetDefaultLanguage(i18n.ZH)
```

### 快速响应

```go
// 响应构建器
builder := i18n.NewResponseBuilder(ctx)
builder.ValidationError("错误")
builder.NotFoundError("错误")
builder.PermissionDeniedError("错误")
builder.ConflictError("错误")
builder.InternalError("错误")

// 便利函数
i18n.BuildValidationErrorResponse(ctx, "")
i18n.BuildNotFoundErrorResponse(ctx, "")
i18n.BuildPermissionDeniedErrorResponse(ctx, "")
i18n.BuildConflictErrorResponse(ctx, "")
i18n.BuildInternalErrorResponse(ctx, "")
```

## 🔢 常用错误码

```go
i18n.CodeOK                     // 200 成功
i18n.CodeBadRequest             // 400 请求错误
i18n.CodeParamError             // 1003 参数错误
i18n.CodeUnauthorized           // 401 未授权
i18n.CodeForbidden              // 403 禁止
i18n.CodeNotFound               // 404 不存在
i18n.CodePermissionDenied       // 1009 权限拒绝
i18n.CodeConflict               // 409 冲突
i18n.CodeInternalError          // 500 内部错误
i18n.CodeServiceUnavailable     // 503 不可用
i18n.CodeUserNotFound           // 1006 用户不存在
i18n.CodeTokenExpired           // 1008 令牌过期
i18n.CodeAuthentication         // 1005 认证必需
```

## 🗣️ 支持的语言

```go
i18n.EN  // "en" - 英文
i18n.ZH  // "zh" - 中文（默认）
```

## 🌐 语言检测

### HTTP 优先级

1. `?lang=en` (query参数)
2. `Accept-Language: en-US` (请求头)
3. 默认语言 (ZH)

### gRPC metadata

```go
// 服务器读取: x-language 或 accept-language
// 客户端拦截器自动传播语言
```

## 📝 在 Handler 中使用

```go
func (h *Handler) GetUser(ctx context.Context, req *GetUserRequest) (*User, error) {
	// Logic处理
	l := logic.NewUserLogic(ctx, h.svc)
	return l.GetUser(ctx, req)
}
```

## 📝 在 Logic 中使用

```go
type UserLogic struct{ ctx context.Context; svc UserService }

func (l *UserLogic) GetUser(ctx context.Context, id string) (*User, error) {
	// 验证
	if id == "" {
		return nil, i18n.BuildValidationErrorResponse(ctx, "ID不能为空")
	}

	// 查询
	user, err := l.svc.GetUser(id)
	if err != nil {
		return nil, i18n.BuildNotFoundErrorResponse(ctx, "用户不存在")
	}

	// 权限检查
	if !l.canAccess(user) {
		return nil, i18n.BuildPermissionDeniedErrorResponse(ctx, "")
	}

	return user, nil
}
```

## ⚙️ 添加自定义错误码

### 1. errors.go

```go
const CodeMyError = 2001
```

### 2. messages.go

```go
MessageMap[2001] = map[Language]string{
	EN: "My error message",
	ZH: "我的错误消息",
}
```

### 3. 使用

```go
i18n.BuildErrorResponse(ctx, CodeMyError, "")
```

## 🧪 在测试中使用

```go
func TestGetUser(t *testing.T) {
	// 中文测试
	ctx := i18n.ContextWithLanguage(context.Background(), i18n.ZH)
	// ... 测试

	// 英文测试
	ctx = i18n.ContextWithLanguage(context.Background(), i18n.EN)
	// ... 测试
}
```

## 📊 错误码对照表

| 代码 | 常量                       | 英文                  | 中文         |
| ---- | -------------------------- | --------------------- | ------------ |
| 200  | CodeOK                     | Ok                    | 成功         |
| 400  | CodeBadRequest             | Parameter error       | 参数错误     |
| 401  | CodeUnauthorized           | Unauthorized          | 未授权       |
| 403  | CodeForbidden              | Forbidden             | 禁止访问     |
| 404  | CodeNotFound               | Not found             | 资源不存在   |
| 409  | CodeConflict               | Conflict              | 冲突         |
| 500  | CodeInternalError          | Internal error        | 服务器错误   |
| 503  | CodeServiceUnavailable     | Service unavailable   | 服务不可用   |
| 1001 | CodeOperationNotAllowed    | Operation not allowed | 操作不被允许 |
| 1002 | CodeResourceExists         | Resource exists       | 资源已存在   |
| 1003 | CodeParamError             | Parameter error       | 参数错误     |
| 1004 | CodeDataValidationError    | Validate error        | 验证错误     |
| 1005 | CodeAuthenticationRequired | Auth required         | 需要认证     |
| 1006 | CodeUserNotFound           | User not found        | 用户不存在   |
| 1007 | CodeInvalidCredentials     | Invalid creds         | 凭证无效     |
| 1008 | CodeTokenExpired           | Token expired         | 令牌过期     |
| 1009 | CodePermissionDenied       | Permission denied     | 权限拒绝     |

## 🔗 完整文档

| 文档                                           | 用途                   |
| ---------------------------------------------- | ---------------------- |
| [README.md](./README.md)                       | 快速开始和API参考      |
| [INTEGRATION_GUIDE.md](./INTEGRATION_GUIDE.md) | 详细集成步骤和最佳实践 |
| [MODULES_OVERVIEW.md](./MODULES_OVERVIEW.md)   | 所有模块详细说明       |
| [examples.go](./examples.go)                   | 10个使用示例           |

## ⚡ 常见模式

### 验证错误

```go
if err := validateRequest(req); err != nil {
	return i18n.BuildValidationErrorResponse(ctx, err.Error())
}
```

### 认证检查

```go
if token == "" {
	return i18n.BuildUnauthorizedErrorResponse(ctx, "")
}
```

### 权限检查

```go
if !hasPermission(user, resource) {
	return i18n.BuildPermissionDeniedErrorResponse(ctx, "")
}
```

### 查询不存在

```go
if data == nil {
	return i18n.BuildNotFoundErrorResponse(ctx, "数据不存在")
}
```

### 冲突/重复

```go
if exists, _ := svc.GetByEmail(email); exists != nil {
	return i18n.BuildConflictErrorResponse(ctx, "邮箱已被使用")
}
```

## 💡 提示

- ✅ **推荐**: 使用 `Build*ErrorResponse` 便利函数
- ✅ **推荐**: 使用 `ResponseBuilder` 进行复杂构建
- ✅ **推荐**: 自定义消息时只使用已存在的错误码
- ✅ **推荐**: 在中间件层自动设置语言

- ❌ **避免**: 硬编码错误消息字符串
- ❌ **避免**: 直接修改 `MessageMap`（在运行时）
- ❌ **避免**: 创建新的错误码而不更新文档
- ❌ **避免**: 忘记注册中间件

## 📞 快速帮助

### 问题: "翻译返回空字符串"

**解决**: 检查错误码是否在 `MessageMap` 中定义

### 问题: "所有请求都是中文，不管语言设置"

**解决**: 确保中间件已正确注册

### 问题: "gRPC客户端语言不传播"

**解决**: 确保客户端也注册了拦截器

### 问题: "性能下降"

**解决**: 翻译性能 <0.1μs，性能问题不在i18n

---

**版本**: 1.0.0  
**最后更新**: 2024年  
**位置**: `/appforge/common/i18n/`
