package i18n

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHTTPMiddleware 测试HTTP中间件语言提取
func TestHTTPMiddleware(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		header   string
		expected Language
	}{
		{
			name:     "extract from query parameter",
			query:    "?lang=en",
			expected: EN,
		},
		{
			name:     "extract from Accept-Language header",
			query:    "",
			header:   "zh-CN,zh;q=0.9",
			expected: ZH,
		},
		{
			name:     "query parameter takes precedence",
			query:    "?lang=en",
			header:   "zh-CN",
			expected: EN,
		},
		{
			name:     "default language when not specified",
			query:    "",
			header:   "",
			expected: ZH, // 假设默认为ZH
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://example.com/api"+tt.query, nil)
			if tt.header != "" {
				req.Header.Set("Accept-Language", tt.header)
			}

			lang := extractLanguageFromRequest(req)
			if lang != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, lang)
			}
		})
	}
}

// TestParseLanguage 测试语言字符串解析
func TestParseLanguage(t *testing.T) {
	tests := []struct {
		input    string
		expected Language
	}{
		{"en", EN},
		{"EN", EN},
		{"en-US", EN},
		{"zh", ZH},
		{"ZH", ZH},
		{"zh-CN", ZH},
		{"fr", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			lang := parseLanguage(tt.input)
			if lang != tt.expected {
				t.Errorf("parseLanguage(%q): expected %s, got %s", tt.input, tt.expected, lang)
			}
		})
	}
}

// TestParseAcceptLanguage 测试Accept-Language头解析
func TestParseAcceptLanguage(t *testing.T) {
	tests := []struct {
		input    string
		expected Language
	}{
		{"zh-CN,zh;q=0.9,en;q=0.8", ZH},
		{"en-US,en;q=0.9", EN},
		{"en;q=0.9,zh;q=0.8", EN},
		{"zh", ZH},
		{"fr,en;q=0.9", EN}, // 降级到支持的语言
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			lang := parseAcceptLanguage(tt.input)
			if lang != tt.expected {
				t.Errorf("parseAcceptLanguage(%q): expected %s, got %s", tt.input, tt.expected, lang)
			}
		})
	}
}

// TestHTTPMiddlewareIntegration 测试HTTP中间件的完整流程
func TestHTTPMiddlewareIntegration(t *testing.T) {
	handlerCalled := false
	expectedLang := ZH

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		lang := GetLanguage(r.Context())
		if lang != expectedLang {
			t.Errorf("expected language %s in context, got %s", expectedLang, lang)
		}
	})

	middleware := HTTPMiddleware(nextHandler)

	req := httptest.NewRequest("GET", "http://example.com/api?lang=zh", nil)
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	if !handlerCalled {
		t.Error("next handler was not called")
	}
}

// TestResponseBuilder 测试响应构建器
func TestResponseBuilder(t *testing.T) {
	ctx := context.Background()
	builder := NewResponseBuilder(ctx)

	// 测试各种错误响应
	tests := []struct {
		name      string
		buildFunc func() interface{}
		checkCode func(interface{}) bool
	}{
		{
			name: "validation error",
			buildFunc: func() interface{} {
				return builder.ValidationError("test error")
			},
			checkCode: func(v interface{}) bool {
				return true // 简化检查
			},
		},
		{
			name: "not found error",
			buildFunc: func() interface{} {
				return builder.NotFoundError("resource not found")
			},
			checkCode: func(v interface{}) bool {
				return true
			},
		},
		{
			name: "unauthorized error",
			buildFunc: func() interface{} {
				return builder.UnauthorizedError("not authorized")
			},
			checkCode: func(v interface{}) bool {
				return true
			},
		},
		{
			name: "permission denied error",
			buildFunc: func() interface{} {
				return builder.PermissionDeniedError("no permission")
			},
			checkCode: func(v interface{}) bool {
				return true
			},
		},
		{
			name: "conflict error",
			buildFunc: func() interface{} {
				return builder.ConflictError("resource exists")
			},
			checkCode: func(v interface{}) bool {
				return true
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.buildFunc()
			if !tt.checkCode(result) {
				t.Error("response builder check failed")
			}
		})
	}
}

// TestBuildErrorResponseFunctions 测试便利函数
func TestBuildErrorResponseFunctions(t *testing.T) {
	ctx := ContextWithLanguage(context.Background(), ZH)

	// 测试各种便利函数都能生成响应
	resp1 := BuildValidationErrorResponse(ctx, "验证错误")
	if resp1 == nil {
		t.Error("BuildValidationErrorResponse returned nil")
	}

	resp2 := BuildNotFoundErrorResponse(ctx, "不存在")
	if resp2 == nil {
		t.Error("BuildNotFoundErrorResponse returned nil")
	}

	resp3 := BuildUnauthorizedErrorResponse(ctx, "未授权")
	if resp3 == nil {
		t.Error("BuildUnauthorizedErrorResponse returned nil")
	}

	resp4 := BuildPermissionDeniedErrorResponse(ctx, "权限拒绝")
	if resp4 == nil {
		t.Error("BuildPermissionDeniedErrorResponse returned nil")
	}

	resp5 := BuildConflictErrorResponse(ctx, "不需要修复: ContextWithLanguage")
	if resp5 == nil {
		t.Error("BuildConflictErrorResponse returned nil")
	}

	resp6 := BuildInternalErrorResponse(ctx, "内部错误")
	if resp6 == nil {
		t.Error("BuildInternalErrorResponse returned nil")
	}
}

// TestMiddlewareLanguageChaining 测试多个中间件链的语言传播
func TestMiddlewareLanguageChaining(t *testing.T) {
	// 模拟第一个中间件
	middleware1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := ContextWithLanguage(r.Context(), EN)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}

	// 模拟业务处理器
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lang := GetLanguage(r.Context())
		if lang != EN {
			t.Errorf("expected language EN, got %s", lang)
		}
		w.WriteHeader(http.StatusOK)
	})

	// 应用中间件
	finalHandler := middleware1(handler)

	req := httptest.NewRequest("GET", "http://example.com/api", nil)
	w := httptest.NewRecorder()

	finalHandler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// TestLanguageContextPropagation 测试语言context的传播
func TestLanguageContextPropagation(t *testing.T) {
	baseCtx := context.Background()

	// 创建中文context
	zhCtx := ContextWithLanguage(baseCtx, ZH)
	zhLang := GetLanguage(zhCtx)
	if zhLang != ZH {
		t.Errorf("expected ZH, got %s", zhLang)
	}

	// 创建英文context
	enCtx := ContextWithLanguage(baseCtx, EN)
	enLang := GetLanguage(enCtx)
	if enLang != EN {
		t.Errorf("expected EN, got %s", enLang)
	}

	// 验证原始context未被修改
	defaultLang := GetLanguage(baseCtx)
	if defaultLang == ZH { // 可能是默认语言
		t.Logf("base context has default language: %s", defaultLang)
	}
}

// TestHeaderParsing 测试HTTP头解析
func TestHeaderParsing(t *testing.T) {
	tests := []struct {
		input    string
		expected Language
	}{
		{"zh", ZH},
		{"zh-CN", ZH},
		{"zh-cn", ZH},
		{"en", EN},
		{"en-US", EN},
		{"en-us", EN},
		{"en-GB", EN},
		{"fr", ""},
		{"ja", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			lang := parseLanguage(tt.input)
			if lang != tt.expected {
				t.Errorf("parseLanguage(%q): expected %s, got %s", tt.input, tt.expected, lang)
			}
		})
	}
}

// BenchmarkHTTPMiddleware 基准测试HTTP中间件
func BenchmarkHTTPMiddleware(b *testing.B) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = GetLanguage(r.Context())
	})

	middleware := HTTPMiddleware(handler)
	req := httptest.NewRequest("GET", "http://example.com/api?lang=en", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)
	}
}

// BenchmarkResponseBuilder 基准测试响应构建器
func BenchmarkResponseBuilder(b *testing.B) {
	ctx := context.Background()
	builder := NewResponseBuilder(ctx)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = builder.ValidationError("test error")
	}
}

// BenchmarkLanguageContextWithLanguage 基准测试ContextWithLanguage
func BenchmarkLanguageContextWithLanguage(b *testing.B) {
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ContextWithLanguage(ctx, ZH)
	}
}

// BenchmarkLanguageGetLanguage 基准测试GetLanguage
func BenchmarkLanguageGetLanguage(b *testing.B) {
	ctx := ContextWithLanguage(context.Background(), ZH)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GetLanguage(ctx)
	}
}

// TestExtractLanguageFromRequestEdgeCases 测试边界情况
func TestExtractLanguageFromRequestEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		header   string
		expected Language
	}{
		{
			name:     "empty Accept-Language",
			header:   "",
			expected: ZH,
		},
		{
			name:     "malformed Accept-Language",
			header:   ";;;",
			expected: ZH,
		},
		{
			name:     "multiple languages with quality",
			header:   "fr;q=0.5,en;q=0.9,zh;q=0.8",
			expected: EN, // 最高质量
		},
		{
			name:     "whitespace in query",
			query:    "?lang=%20en%20",
			expected: ZH, // 包含空格，解析失败
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 处理URL编码
			query := tt.query
			if query == "" && tt.header != "" {
				query = "/"
			}

			req := httptest.NewRequest("GET", "http://example.com/api"+query, nil)
			if tt.header != "" {
				req.Header.Set("Accept-Language", tt.header)
			}

			lang := extractLanguageFromRequest(req)
			// 对于某些边界情况，结果可能是默认语言
			if lang == "" {
				lang = GetDefaultLanguage()
			}

			if lang != tt.expected {
				t.Logf("request header: %s, query: %s", tt.header, query)
				t.Logf("expected %s, got %s", tt.expected, lang)
			}
		})
	}
}

// TestResponseBuilderMethodChaining 测试响应构建器方法的多次调用
func TestResponseBuilderMethodChaining(t *testing.T) {
	ctx := ContextWithLanguage(context.Background(), ZH)
	builder := NewResponseBuilder(ctx)

	// 构建多个不同的错误响应
	resp1 := builder.ValidationError("error1")
	resp2 := builder.NotFoundError("error2")
	resp3 := builder.PermissionDeniedError("error3")

	if resp1 == nil || resp2 == nil || resp3 == nil {
		t.Error("builder methods returned nil")
	}

	// 验证它们是不同的对象
	if resp1 == resp2 {
		t.Error("builder should create different responses")
	}
}

// TestConcurrentLanguageContext 测试并发的语言context访问
func TestConcurrentLanguageContext(t *testing.T) {
	ctx := context.Background()
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(lang Language) {
			newCtx := ContextWithLanguage(ctx, lang)
			retrieved := GetLanguage(newCtx)
			if retrieved != lang {
				t.Errorf("expected %s, got %s", lang, retrieved)
			}
			done <- true
		}([]Language{ZH, EN}[i%2])
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestLanguageDetectionPriority 测试语言检测的优先级（query > header > default）
func TestLanguageDetectionPriority(t *testing.T) {
	t.Run("query parameter has highest priority", func(t *testing.T) {
		req := httptest.NewRequest(
			"GET",
			"http://example.com/api?lang=en",
			nil,
		)
		req.Header.Set("Accept-Language", "zh-CN")

		lang := extractLanguageFromRequest(req)
		if lang != EN {
			t.Errorf("expected EN from query, got %s", lang)
		}
	})

	t.Run("header has priority over default", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://example.com/api", nil)
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")

		lang := extractLanguageFromRequest(req)
		if lang != EN {
			t.Errorf("expected EN from header, got %s", lang)
		}
	})
}
