package i18n

import (
	"context"
	"appforge/common/helper"
	"appforge/proto/common"
)

// ResponseBuilder 用于方便地构建带有翻译消息的响应
type ResponseBuilder struct {
	ctx context.Context
}

// NewResponseBuilder 创建响应构建器
func NewResponseBuilder(ctx context.Context) *ResponseBuilder {
	return &ResponseBuilder{ctx: ctx}
}

// ErrorResponse 构建错误响应
func (rb *ResponseBuilder) ErrorResponse(code int32, customMsg string) *common.RespBase {
	msg := TranslateError(code, customMsg, rb.ctx)
	return helper.ErrResp(code, msg)
}

// SuccessResponse 构建成功响应
func (rb *ResponseBuilder) SuccessResponse() *common.RespBase {
	return helper.OkResp()
}

// ValidationError 构建验证错误响应
func (rb *ResponseBuilder) ValidationError(message string) *common.RespBase {
	msg := TranslateError(CodeDataValidationError, message, rb.ctx)
	return helper.ErrResp(CodeDataValidationError, msg)
}

// NotFoundError 构建资源不存在响应
func (rb *ResponseBuilder) NotFoundError(message string) *common.RespBase {
	msg := TranslateError(CodeNotFound, message, rb.ctx)
	return helper.ErrResp(CodeNotFound, msg)
}

// UnauthorizedError 构建未授权响应
func (rb *ResponseBuilder) UnauthorizedError(message string) *common.RespBase {
	msg := TranslateError(CodeUnauthorized, message, rb.ctx)
	return helper.ErrResp(CodeUnauthorized, msg)
}

// ForbiddenError 构建禁止访问响应
func (rb *ResponseBuilder) ForbiddenError(message string) *common.RespBase {
	msg := TranslateError(CodeForbidden, message, rb.ctx)
	return helper.ErrResp(CodeForbidden, msg)
}

// PermissionDeniedError 构建权限被拒绝响应
func (rb *ResponseBuilder) PermissionDeniedError(message string) *common.RespBase {
	msg := TranslateError(CodePermissionDenied, message, rb.ctx)
	return helper.ErrResp(CodePermissionDenied, msg)
}

// ConflictError 构建冲突响应
func (rb *ResponseBuilder) ConflictError(message string) *common.RespBase {
	msg := TranslateError(CodeConflict, message, rb.ctx)
	return helper.ErrResp(CodeConflict, msg)
}

// InternalError 构建服务器内部错误响应
func (rb *ResponseBuilder) InternalError(message string) *common.RespBase {
	msg := TranslateError(CodeInternalError, message, rb.ctx)
	return helper.ErrResp(CodeInternalError, msg)
}

// 便利函数 - 直接使用全局翻译器

// BuildErrorResponse 构建翻译的错误响应
func BuildErrorResponse(ctx context.Context, code int32, customMsg string) *common.RespBase {
	msg := TranslateError(code, customMsg, ctx)
	return helper.ErrResp(code, msg)
}

// BuildValidationErrorResponse 构建验证错误响应
func BuildValidationErrorResponse(ctx context.Context, message string) *common.RespBase {
	msg := TranslateError(CodeDataValidationError, message, ctx)
	return helper.ErrResp(CodeDataValidationError, msg)
}

// BuildNotFoundErrorResponse 构建资源不存在响应
func BuildNotFoundErrorResponse(ctx context.Context, message string) *common.RespBase {
	msg := TranslateError(CodeNotFound, message, ctx)
	return helper.ErrResp(CodeNotFound, msg)
}

// BuildUnauthorizedErrorResponse 构建未授权响应
func BuildUnauthorizedErrorResponse(ctx context.Context, message string) *common.RespBase {
	msg := TranslateError(CodeUnauthorized, message, ctx)
	return helper.ErrResp(CodeUnauthorized, msg)
}

// BuildPermissionDeniedErrorResponse 构建权限被拒绝响应
func BuildPermissionDeniedErrorResponse(ctx context.Context, message string) *common.RespBase {
	msg := TranslateError(CodePermissionDenied, message, ctx)
	return helper.ErrResp(CodePermissionDenied, msg)
}

// BuildConflictErrorResponse 构建冲突响应
func BuildConflictErrorResponse(ctx context.Context, message string) *common.RespBase {
	msg := TranslateError(CodeConflict, message, ctx)
	return helper.ErrResp(CodeConflict, msg)
}

// BuildInternalErrorResponse 构建服务器内部错误响应
func BuildInternalErrorResponse(ctx context.Context, message string) *common.RespBase {
	msg := TranslateError(CodeInternalError, message, ctx)
	return helper.ErrResp(CodeInternalError, msg)
}
