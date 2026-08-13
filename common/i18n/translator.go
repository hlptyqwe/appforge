package i18n

import (
	"context"
	"sync"
)

// Translator 翻译器实例
type Translator struct {
	defaultLang Language
	mu          sync.RWMutex
}

// contextKey 用于在context中存储语言
type contextKey string

const langKey contextKey = "language"

// 全局翻译器实例
var globalTranslator *Translator

func init() {
	globalTranslator = &Translator{
		defaultLang: ZH, // 默认使用中文
	}
}

// NewTranslator 创建新的翻译器
func NewTranslator(defaultLang Language) *Translator {
	return &Translator{
		defaultLang: defaultLang,
	}
}

// SetDefaultLanguage 设置默认语言
func (t *Translator) SetDefaultLanguage(lang Language) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.defaultLang = lang
}

// GetLanguage 从context获取语言，若不存在则使用默认语言
func (t *Translator) GetLanguage(ctx context.Context) Language {
	if lang, ok := ctx.Value(langKey).(Language); ok {
		return lang
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.defaultLang
}

// Translate 翻译错误/消息代码
// code: 错误代码
// ctx: 上下文，用于获取语言设置
func (t *Translator) Translate(code int32, ctx context.Context) string {
	lang := t.GetLanguage(ctx)
	if messages, ok := MessageMap[code]; ok {
		if msg, ok := messages[lang]; ok {
			return msg
		}
		// 如果指定语言的翻译不存在，回退到英文
		if msg, ok := messages[EN]; ok {
			return msg
		}
	}
	// 如果都没有找到，返回默认消息
	return "Unknown error"
}

// TranslateWithDefault 翻译错误/消息代码，如果不存在则返回默认消息
func (t *Translator) TranslateWithDefault(code int32, defaultMsg string, ctx context.Context) string {
	lang := t.GetLanguage(ctx)
	if messages, ok := MessageMap[code]; ok {
		if msg, ok := messages[lang]; ok {
			return msg
		}
		if msg, ok := messages[EN]; ok {
			return msg
		}
	}
	return defaultMsg
}

// WithLanguage 创建包含语言信息的context
func WithLanguage(ctx context.Context, lang Language) context.Context {
	return context.WithValue(ctx, langKey, lang)
}

// ========== 全局函数 ==========

// Translate 使用全局翻译器翻译
func Translate(code int32, ctx context.Context) string {
	return globalTranslator.Translate(code, ctx)
}

// TranslateWithDefault 使用全局翻译器翻译，带默认值
func TranslateWithDefault(code int32, defaultMsg string, ctx context.Context) string {
	return globalTranslator.TranslateWithDefault(code, defaultMsg, ctx)
}

// SetDefaultLanguage 设置全局翻译器的默认语言
func SetDefaultLanguage(lang Language) {
	globalTranslator.SetDefaultLanguage(lang)
}

// WithLanguage 创建包含语言信息的context
func ContextWithLanguage(ctx context.Context, lang Language) context.Context {
	return WithLanguage(ctx, lang)
}

// GetLanguage 从context获取语言，若不存在则使用默认语言
func GetLanguage(ctx context.Context) Language {
	return globalTranslator.GetLanguage(ctx)
}

// GetDefaultLanguage 获取全局默认语言
func GetDefaultLanguage() Language {
	globalTranslator.mu.RLock()
	defer globalTranslator.mu.RUnlock()
	return globalTranslator.defaultLang
}
