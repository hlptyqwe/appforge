package i18n

import (
	"context"
	"testing"
)

func TestTranslate(t *testing.T) {
	cases := []struct {
		name     string
		code     int32
		lang     Language
		expected string
	}{
		{
			name:     "ParamError in Chinese",
			code:     ParamError,
			lang:     ZH,
			expected: "参数错误",
		},
		{
			name:     "ParamError in English",
			code:     ParamError,
			lang:     EN,
			expected: "Parameter error",
		},
		{
			name:     "UserNotFound in Chinese",
			code:     UserNotFound,
			lang:     ZH,
			expected: "用户不存在",
		},
		{
			name:     "UserNotFound in English",
			code:     UserNotFound,
			lang:     EN,
			expected: "User not found",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := ContextWithLanguage(context.Background(), tc.lang)
			result := Translate(tc.code, ctx)
			if result != tc.expected {
				t.Errorf("Translate(%d, %s) = %q; want %q", tc.code, tc.lang, result, tc.expected)
			}
		})
	}
}

func TestTranslateWithDefault(t *testing.T) {
	ctx := ContextWithLanguage(context.Background(), ZH)

	// Test with custom message
	result := TranslateError(UserNotFound, "自定义消息", ctx)
	if result != "自定义消息" {
		t.Errorf("TranslateError with custom msg = %q; want %q", result, "自定义消息")
	}

	// Test without custom message
	result = TranslateError(UserNotFound, "", ctx)
	if result != "用户不存在" {
		t.Errorf("TranslateError without custom msg = %q; want %q", result, "用户不存在")
	}
}

func TestDefaultLanguage(t *testing.T) {
	ctx := context.Background()

	// Should use Chinese by default
	result := Translate(ParamError, ctx)
	if result != "参数错误" {
		t.Errorf("Default language translation = %q; want %q", result, "参数错误")
	}
}

func TestSetDefaultLanguage(t *testing.T) {
	oldLang := globalTranslator.defaultLang
	defer func() {
		globalTranslator.SetDefaultLanguage(oldLang)
	}()

	// Change default language to English
	SetDefaultLanguage(EN)
	ctx := context.Background()

	result := Translate(ParamError, ctx)
	if result != "Parameter error" {
		t.Errorf("After SetDefaultLanguage(EN) = %q; want %q", result, "Parameter error")
	}
}

func TestContextLanguageOverride(t *testing.T) {
	// Set default to Chinese
	SetDefaultLanguage(ZH)

	// But context specifies English
	ctx := ContextWithLanguage(context.Background(), EN)
	result := Translate(ParamError, ctx)

	if result != "Parameter error" {
		t.Errorf("Context override = %q; want %q", result, "Parameter error")
	}
}

func TestFallbackLanguage(t *testing.T) {
	ctx := ContextWithLanguage(context.Background(), Language("invalid"))
	result := Translate(OK, ctx)

	// Should fallback to English for valid codes
	if result != "OK" && result != "成功" {
		t.Errorf("Fallback = %q; want either 'OK' or '成功'", result)
	}
}

func TestUnknownCode(t *testing.T) {
	ctx := context.Background()
	result := Translate(9999, ctx)

	if result == "" {
		t.Errorf("Unknown code returned empty string")
	}
}

func TestErrorInfo(t *testing.T) {
	// Test with code only
	errInfo := NewErrorInfo(ParamError, "")
	ctx := ContextWithLanguage(context.Background(), ZH)
	if errInfo.GetMessage(ctx) != "参数错误" {
		t.Errorf("ErrorInfo with code only = %q", errInfo.GetMessage(ctx))
	}

	// Test with custom message
	errInfo = NewErrorInfo(ParamError, "自定义错误")
	if errInfo.GetMessage(ctx) != "自定义错误" {
		t.Errorf("ErrorInfo with custom message = %q", errInfo.GetMessage(ctx))
	}
}

func BenchmarkTranslate(b *testing.B) {
	ctx := ContextWithLanguage(context.Background(), ZH)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Translate(ParamError, ctx)
	}
}
