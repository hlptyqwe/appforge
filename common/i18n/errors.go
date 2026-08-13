package i18n

import (
	"context"
)

// ErrorInfo 包含错误码和消息
type ErrorInfo struct {
	Code    int32
	Message string
}

// NewErrorInfo 创建新的错误信息
func NewErrorInfo(code int32, message string) *ErrorInfo {
	return &ErrorInfo{
		Code:    code,
		Message: message,
	}
}

// GetMessage 获取翻译后的消息
func (e *ErrorInfo) GetMessage(ctx context.Context) string {
	// 如果有自定义消息，优先使用自定义消息
	if e.Message != "" {
		return e.Message
	}
	// 否则使用翻译的消息
	return Translate(e.Code, ctx)
}

// TranslateError 翻译错误码为错误消息
// 如果提供了 customMsg（非空），则使用该自定义消息
// 否则使用翻译字典中的消息
func TranslateError(code int32, customMsg string, ctx context.Context) string {
	if customMsg != "" {
		return customMsg
	}
	return Translate(code, ctx)
}

// 常用错误码常量（为了方便使用）
const (
	// 成功
	CodeOK = 200

	// 客户端错误 (4xx)
	CodeBadRequest          = 400  // 参数错误
	CodeUnauthorized        = 401  // 未授权
	CodeForbidden           = 403  // 禁止访问
	CodeNotFound            = 404  // 资源不存在
	CodeConflict            = 409  // 冲突
	CodeParamError          = 1003 // 参数验证错误
	CodeDataValidationError = 1004 // 数据验证错误

	// 认证相关
	CodeAuthenticationRequired = 1005 // 需要认证
	CodeUserNotFound           = 1006 // 用户不存在
	CodeInvalidCredentials     = 1007 // 凭证无效
	CodeTokenExpired           = 1008 // 令牌过期

	// 权限相关
	CodePermissionDenied = 1009 // 权限被拒绝

	// 业务相关
	CodeOperationNotAllowed = 1001 // 操作不被允许
	CodeResourceExists      = 1002 // 资源已存在

	// 服务器错误 (5xx)
	CodeInternalError      = 500 // 服务器内部错误
	CodeServiceUnavailable = 503 // 服务不可用
)
