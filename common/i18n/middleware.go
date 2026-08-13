package i18n

import (
	"context"
	"net/http"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// HTTPMiddleware 为HTTP请求添加语言支持的中间件
// 从Accept-Language头或query参数提取语言
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lang := extractLanguageFromRequest(r)
		ctx := ContextWithLanguage(r.Context(), lang)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractLanguageFromRequest 从HTTP请求中提取语言
// 优先级: query参数 lang > Accept-Language header > 默认语言
func extractLanguageFromRequest(r *http.Request) Language {
	// 1. 检查 query 参数 ?lang=en
	if langParam := r.URL.Query().Get("lang"); langParam != "" {
		if lang := parseLanguage(langParam); lang != "" {
			return lang
		}
	}

	// 2. 检查 Accept-Language header
	// 例如: Accept-Language: zh-CN,zh;q=0.9,en;q=0.8
	if acceptLang := r.Header.Get("Accept-Language"); acceptLang != "" {
		if lang := parseAcceptLanguage(acceptLang); lang != "" {
			return lang
		}
	}

	// 3. 返回默认语言
	return GetDefaultLanguage()
}

// parseLanguage 将语言字符串解析为支持的语言
func parseLanguage(langStr string) Language {
	langStr = strings.TrimSpace(strings.ToLower(langStr))

	// 支持多种格式: "en", "en-US", "zh", "zh-CN"
	switch {
	case langStr == "en" || strings.HasPrefix(langStr, "en-"):
		return EN
	case langStr == "zh" || strings.HasPrefix(langStr, "zh-"):
		return ZH
	default:
		return ""
	}
}

// parseAcceptLanguage 从Accept-Language头解析最优先的语言
// 格式: zh-CN,zh;q=0.9,en;q=0.8
func parseAcceptLanguage(acceptLang string) Language {
	parts := strings.Split(acceptLang, ",")

	var selectedLang Language
	var highestQ float64 = -1

	for _, part := range parts {
		part = strings.TrimSpace(part)

		// 分解 language;q=quality
		langPart := part
		quality := 1.0

		if idx := strings.Index(part, ";q="); idx != -1 {
			langPart = part[:idx]
			// 简单解析quality值
			qStr := part[idx+3:]
			if q, err := parseQuality(qStr); err == nil {
				quality = q
			}
		}

		if quality > highestQ {
			if lang := parseLanguage(langPart); lang != "" {
				selectedLang = lang
				highestQ = quality
			}
		}
	}

	return selectedLang
}

// parseQuality 解析 quality 值
func parseQuality(qStr string) (float64, error) {
	qStr = strings.TrimSpace(qStr)
	var q float64
	_, err := strings.NewReader(qStr).Read([]byte{})
	if err != nil {
		return 1.0, err
	}

	// 简单的 q 值提取 (0.0-1.0)
	q = 1.0
	if strings.HasPrefix(qStr, "0") {
		if len(qStr) >= 3 {
			switch qStr[2] {
			case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
				q = 0.9 // 简化处理
			}
		}
	}
	return q, nil
}

// GRPCUnaryServerInterceptor gRPC一元调用的语言支持拦截器
func GRPCUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		ctx = extractLanguageFromGRPCMetadata(ctx)
		return handler(ctx, req)
	}
}

// GRPCStreamServerInterceptor gRPC流调用的语言支持拦截器
func GRPCStreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx := extractLanguageFromGRPCMetadata(ss.Context())
		return handler(srv, &contextStream{ServerStream: ss, ctx: ctx})
	}
}

// extractLanguageFromGRPCMetadata 从gRPC metadata中提取语言
func extractLanguageFromGRPCMetadata(ctx context.Context) context.Context {
	// 获取gRPC metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}

	// 1. 检查 x-language header
	if langs := md.Get("x-language"); len(langs) > 0 {
		if lang := parseLanguage(langs[0]); lang != "" {
			return ContextWithLanguage(ctx, lang)
		}
	}

	// 2. 检查 accept-language header
	if langs := md.Get("accept-language"); len(langs) > 0 {
		if lang := parseAcceptLanguage(langs[0]); lang != "" {
			return ContextWithLanguage(ctx, lang)
		}
	}

	return ctx
}

// contextStream 包装grpc.ServerStream以支持自定义context
type contextStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (cs *contextStream) Context() context.Context {
	return cs.ctx
}

// GRPCClientUnaryInterceptor gRPC客户端一元调用的语言传播拦截器
func GRPCClientUnaryInterceptor() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		// 如果context中有语言，将其添加到metadata
		if lang := GetLanguage(ctx); lang != "" && lang != GetDefaultLanguage() {
			ctx = metadata.AppendToOutgoingContext(ctx, "x-language", string(lang))
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// GRPCClientStreamInterceptor gRPC客户端流调用的语言传播拦截器
func GRPCClientStreamInterceptor() grpc.StreamClientInterceptor {
	return func(
		ctx context.Context,
		desc *grpc.StreamDesc,
		cc *grpc.ClientConn,
		method string,
		streamer grpc.Streamer,
		opts ...grpc.CallOption,
	) (grpc.ClientStream, error) {
		// 如果context中有语言，将其添加到metadata
		if lang := GetLanguage(ctx); lang != "" && lang != GetDefaultLanguage() {
			ctx = metadata.AppendToOutgoingContext(ctx, "x-language", string(lang))
		}
		return streamer(ctx, desc, cc, method, opts...)
	}
}
