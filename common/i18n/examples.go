package i18n

import (
	"context"
	"appforge/common/helper"
	"appforge/proto/common"
)

// ===== 使用示例 =====
// 本文件展示如何在实际业务逻辑中使用i18n模块
// 删除本文件后不影响功能

/*

========== 示例 1: 在 Logic 层处理错误 ==========

package logic

import (
	"context"
	"appforge/common/i18n"
)

func (l *SomeLogic) SomeOperation(ctx context.Context, req *SomeRequest) (*SomeResponse, error) {
	// 参数验证
	if req.Name == "" {
		resp := i18n.BuildValidationErrorResponse(ctx, "名称不能为空")
		return nil, resp
	}

	// 查询数据
	data, err := l.svc.GetData(req.Id)
	if err != nil {
		if err == ErrNotFound {
			resp := i18n.BuildNotFoundErrorResponse(ctx, "数据不存在")
			return nil, resp
		}
		resp := i18n.BuildInternalErrorResponse(ctx, err.Error())
		return nil, resp
	}

	// 权限检查
	if !l.hasPermission(ctx, data) {
		resp := i18n.BuildPermissionDeniedErrorResponse(ctx, "没有权限访问此数据")
		return nil, resp
	}

	// 业务逻辑
	result := l.processData(data)

	return &SomeResponse{
		Data: result,
	}, nil
}

========== 示例 2: 使用 ResponseBuilder ==========

package logic

import (
	"context"
	"appforge/common/i18n"
)

type UserLogic struct {
	ctx context.Context
	svc UserService
}

func (l *UserLogic) CreateUser(ctx context.Context, req *CreateUserRequest) (*UserResponse, error) {
	builder := i18n.NewResponseBuilder(ctx)

	// 验证
	if err := l.validateRequest(req); err != nil {
		return nil, builder.ValidationError(err.Error())
	}

	// 检查是否存在
	existing, err := l.svc.FindByEmail(req.Email)
	if err != nil {
		return nil, builder.InternalError(err.Error())
	}
	if existing != nil {
		return nil, builder.ConflictError("邮箱已被注册")
	}

	// 创建用户
	user, err := l.svc.Create(req)
	if err != nil {
		return nil, builder.InternalError(err.Error())
	}

	return &UserResponse{User: user}, nil
}

========== 示例 3: 自定义错误码 ==========

// 在 errors.go 中添加自定义错误码
const (
	CodeUserNotVerified = 2001
	CodePaymentFailed   = 2002
	CodeInsufficientBalance = 2003
)

// 在 messages.go 中添加对应的翻译
import "sync"

var (
	customMessages = map[int32]map[Language]string{
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
	}
	customMessagesMutex sync.Mutex
)

// 在init中初始化
func init() {
	// 将自定义消息合并到全局映射
	customMessagesMutex.Lock()
	defer customMessagesMutex.Unlock()

	for code, langs := range customMessages {
		if _, ok := MessageMap[code]; ok {
			// 不覆盖已存在的码
			continue
		}
		MessageMap[code] = langs
	}
}

========== 示例 4: 在 Handler 层使用 ==========

package handler

import (
	"appforge/common/i18n"
	"appforge/yourservice/internal/logic"
	"context"
)

func (h *Handler) CreateUserHandler(ctx context.Context, req *CreateUserRequest) (*Response, error) {
	l := logic.NewUserLogic(ctx, h.svc)

	result, err := l.CreateUser(ctx, req)
	if err != nil {
		// 错误已包含翻译
		return nil, err
	}

	return &Response{
		Code: 200,
		Msg:  i18n.Translate(i18n.CodeOK, ctx),
		Data: result,
	}, nil
}

========== 示例 5: 在中间件中设置语言 ==========

// HTTP 中间件示例
package middleware

import (
	"net/http"
	"appforge/common/i18n"
)

func LanguageMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// i18n.HTTPMiddleware 已提供此功能
		// 这是其实现方式的展示

		lang := "zh" // 从请求头或参数获取
		if lang == "en" {
			r = r.WithContext(
				i18n.ContextWithLanguage(r.Context(), i18n.EN),
			)
		} else {
			r = r.WithContext(
				i18n.ContextWithLanguage(r.Context(), i18n.ZH),
			)
		}

		next.ServeHTTP(w, r)
	})
}

// gRPC 拦截器示例
package interceptor

import (
	"context"
	"google.golang.org/grpc"
	"appforge/common/i18n"
)

func LanguageInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	// i18n.GRPCUnaryServerInterceptor() 已提供此功能
	// 这展示了如何使用

	lang := i18n.GetLanguage(ctx)
	if lang == "" {
		lang = i18n.ZH // 设置默认语言
		ctx = i18n.ContextWithLanguage(ctx, lang)
	}

	return handler(ctx, req)
}

========== 示例 6: 处理未知错误码 ==========

package logic

import (
	"context"
	"appforge/common/i18n"
)

func (l *SomeLogic) HandleError(ctx context.Context, code int32, customMsg string) error {
	// TranslateError 会提供三级降级:
	// 1. 使用自定义消息（如果提供）
	// 2. 翻译对应语言的错误码
	// 3. 降级到英文
	// 4. 如果码不存在，返回空字符串

	msg := i18n.TranslateError(code, customMsg, ctx)
	if msg == "" {
		// 码不存在，记录并使用通用错误
		l.logError("unknown error code", code)
		msg = i18n.Translate(i18n.CodeInternalError, ctx)
	}

	resp := i18n.BuildErrorResponse(ctx, code, customMsg)
	return resp
}

========== 示例 7: 在服务启动时配置 ==========

package main

import (
	"appforge/common/i18n"
)

func init() {
	// 设置全局默认语言为中文
	i18n.SetDefaultLanguage(i18n.ZH)

	// 可以根据环境变量配置
	// if os.Getenv("DEFAULT_LANG") == "en" {
	//     i18n.SetDefaultLanguage(i18n.EN)
	// }
}

========== 示例 8: 列表查询返回多语言消息 ==========

package logic

import (
	"context"
	"appforge/common/i18n"
)

func (l *DataLogic) ListUsers(ctx context.Context, req *ListRequest) (*ListResponse, error) {
	// 获取数据
	items, total, err := l.svc.List(req.Page, req.Size)
	if err != nil {
		resp := i18n.BuildInternalErrorResponse(ctx, err.Error())
		return nil, resp
	}

	// 返回成功响应，消息已翻译
	return &ListResponse{
		Code:  200,
		Msg:   i18n.Translate(i18n.CodeOK, ctx),
		Data:  items,
		Total: total,
	}, nil
}

========== 示例 9: 处理权限和认证错误 ==========

package logic

import (
	"context"
	"appforge/common/i18n"
)

func (l *AuthLogic) CheckAuth(ctx context.Context, token string) error {
	if token == "" {
		// AuthenticationRequired (1005)
		msg := i18n.Translate(i18n.CodeAuthenticationRequired, ctx)
		return i18n.BuildErrorResponse(ctx, i18n.CodeAuthenticationRequired, msg)
	}

	user, err := l.ValidateToken(token)
	if err != nil {
		if err == ErrTokenExpired {
			// TokenExpired (1008)
			msg := i18n.Translate(i18n.CodeTokenExpired, ctx)
			return i18n.BuildErrorResponse(ctx, i18n.CodeTokenExpired, msg)
		}

		// InvalidCredentials (1007)
		msg := i18n.Translate(i18n.CodeInvalidCredentials, ctx)
		return i18n.BuildErrorResponse(ctx, i18n.CodeInvalidCredentials, msg)
	}

	return nil
}

========== 示例 10: 完整的业务逻辑示例 ==========

package logic

import (
	"context"
	"appforge/common/i18n"
)

type PaymentLogic struct {
	svc PaymentService
	db  Database
}

func (l *PaymentLogic) ProcessPayment(ctx context.Context, req *PaymentRequest) (*PaymentResponse, error) {
	builder := i18n.NewResponseBuilder(ctx)

	// 1. 参数验证
	if req.Amount <= 0 {
		return nil, builder.ValidationError("金额必须大于0")
	}

	// 2. 权限检查
	user, err := l.getCurrentUser(ctx)
	if err != nil {
		return nil, builder.UnauthorizedError("用户未登录")
	}

	// 3. 查询数据
	order, err := l.svc.GetOrder(req.OrderID)
	if err != nil {
		return nil, builder.NotFoundError("订单不存在")
	}

	// 4. 余额检查
	if user.Balance < req.Amount {
		// 使用自定义业务错误码
		msg := i18n.TranslateError(CodeInsufficientBalance, "", ctx)
		return nil, i18n.BuildErrorResponse(ctx, CodeInsufficientBalance, "")
	}

	// 5. 处理支付
	result, err := l.svc.Pay(order, req.Amount)
	if err != nil {
		// 支付失败
		msg := i18n.TranslateError(CodePaymentFailed, err.Error(), ctx)
		return nil, i18n.BuildErrorResponse(ctx, CodePaymentFailed, err.Error())
	}

	// 6. 返回成功响应
	return &PaymentResponse{
		OrderID: order.ID,
		Amount:  result.Amount,
	}, nil
}

*/

// Example_ResponseBuilder 展示 ResponseBuilder 的使用
func Example_ResponseBuilder() *common.RespBase {
	ctx := context.Background()
	builder := NewResponseBuilder(ctx)

	// 直接构建错误响应
	return builder.ValidationError("示例错误消息")
}

// Example_DirectFunctions 展示直接使用便利函数
func Example_DirectFunctions() *common.RespBase {
	ctx := context.Background()

	// 方式1: 使用便利函数
	resp := BuildValidationErrorResponse(ctx, "参数验证失败")

	// 方式2: 直接使用转换和GetErrResp
	msg := Translate(CodeDataValidationError, ctx)
	resp = helper.ErrResp(CodeDataValidationError, msg)

	return resp
}

// Example_LanguageSelection 展示语言选择
func Example_LanguageSelection() {
	// 中文context
	ctxZH := ContextWithLanguage(context.Background(), ZH)
	msgZH := Translate(CodeParamError, ctxZH) // "参数错误"

	// 英文context
	ctxEN := ContextWithLanguage(context.Background(), EN)
	msgEN := Translate(CodeParamError, ctxEN) // "Parameter error"

	// 系统默认（初始为中文）
	msgDefault := Translate(CodeParamError, context.Background()) // "参数错误"

	_, _, _ = msgZH, msgEN, msgDefault
}

// Example_CustomMessage 展示使用自定义消息
func Example_CustomMessage() *common.RespBase {
	ctx := context.Background()

	// 使用自定义消息覆盖翻译
	msg := TranslateError(CodeParamError, "用户输入的特定参数无效", ctx)
	// 返回: "用户输入的特定参数无效" (自定义消息优先)

	resp := helper.ErrResp(CodeParamError, msg)

	return resp
}
