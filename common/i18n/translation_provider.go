package i18n

import (
	"sync"
)

// TranslationProvider 翻译提供者接口（支持不同的翻译源）
type TranslationProvider interface {
	// GetMessage 获取指定语言和错误码的消息
	GetMessage(lang Language, code int32) string

	// SetMessage 设置翻译（支持热更新）
	SetMessage(lang Language, code int32, message string)

	// HasMessage 检查是否有指定的翻译
	HasMessage(lang Language, code int32) bool
}

// MemoryTranslationProvider 基于内存的翻译提供者（默认实现）
type MemoryTranslationProvider struct {
	// 改进的结构：按语言组织翻译
	// map[Language]map[int32]string
	messages map[Language]map[int32]string
	mu       sync.RWMutex
}

// NewMemoryTranslationProvider 创建内存翻译提供者
func NewMemoryTranslationProvider() *MemoryTranslationProvider {
	return &MemoryTranslationProvider{
		messages: make(map[Language]map[int32]string),
	}
}

// GetMessage 获取翻译
func (p *MemoryTranslationProvider) GetMessage(lang Language, code int32) string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if messages, ok := p.messages[lang]; ok {
		if msg, ok := messages[code]; ok {
			return msg
		}
	}
	return ""
}

// SetMessage 设置翻译
func (p *MemoryTranslationProvider) SetMessage(lang Language, code int32, message string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.messages[lang]; !ok {
		p.messages[lang] = make(map[int32]string)
	}
	p.messages[lang][code] = message
}

// HasMessage 检查是否有翻译
func (p *MemoryTranslationProvider) HasMessage(lang Language, code int32) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if messages, ok := p.messages[lang]; ok {
		_, ok := messages[code]
		return ok
	}
	return false
}

// LoadMessages 批量加载消息（便利方法）
func (p *MemoryTranslationProvider) LoadMessages(messages map[Language]map[int32]string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for lang, msgs := range messages {
		if _, ok := p.messages[lang]; !ok {
			p.messages[lang] = make(map[int32]string)
		}
		for code, msg := range msgs {
			p.messages[lang][code] = msg
		}
	}
}

// CachedTranslationProvider 支持多层降级的翻译提供者
type CachedTranslationProvider struct {
	primary   TranslationProvider // 主翻译源
	fallback  TranslationProvider // 降级翻译源
	cache     map[string]string   // 缓存 (lang_code -> message)
	mu        sync.RWMutex
	cacheHits int64
}

// NewCachedTranslationProvider 创建缓存翻译提供者
func NewCachedTranslationProvider(primary, fallback TranslationProvider) *CachedTranslationProvider {
	return &CachedTranslationProvider{
		primary:  primary,
		fallback: fallback,
		cache:    make(map[string]string),
	}
}

// GetMessage 获取翻译（带缓存和降级）
func (p *CachedTranslationProvider) GetMessage(lang Language, code int32) string {
	cacheKey := string(lang) + "_" + string(rune(code))

	// 先查缓存
	p.mu.RLock()
	if msg, ok := p.cache[cacheKey]; ok {
		p.mu.RUnlock()
		return msg
	}
	p.mu.RUnlock()

	// 查主源
	msg := p.primary.GetMessage(lang, code)

	// 如果主源没有，查降级源
	if msg == "" && p.fallback != nil {
		msg = p.fallback.GetMessage(lang, code)
	}

	// 缓存结果
	p.mu.Lock()
	p.cache[cacheKey] = msg
	p.mu.Unlock()

	return msg
}

// SetMessage 设置翻译（清除缓存）
func (p *CachedTranslationProvider) SetMessage(lang Language, code int32, message string) {
	cacheKey := string(lang) + "_" + string(rune(code))

	p.mu.Lock()
	defer p.mu.Unlock()

	p.primary.SetMessage(lang, code, message)
	delete(p.cache, cacheKey) // 清除缓存
}

// HasMessage 检查翻译
func (p *CachedTranslationProvider) HasMessage(lang Language, code int32) bool {
	msg := p.GetMessage(lang, code)
	return msg != ""
}

// ClearCache 清除所有缓存
func (p *CachedTranslationProvider) ClearCache() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cache = make(map[string]string)
}

// GetCacheStats 获取缓存统计
func (p *CachedTranslationProvider) GetCacheStats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return map[string]interface{}{
		"cache_size": len(p.cache),
		"hits":       p.cacheHits,
	}
}

// MultiSourceTranslationProvider 支持多源翻译的提供者
type MultiSourceTranslationProvider struct {
	providers []TranslationProvider
	mu        sync.RWMutex
}

// NewMultiSourceTranslationProvider 创建多源翻译提供者
func NewMultiSourceTranslationProvider(providers ...TranslationProvider) *MultiSourceTranslationProvider {
	return &MultiSourceTranslationProvider{
		providers: providers,
	}
}

// GetMessage 依次查询各个源
func (p *MultiSourceTranslationProvider) GetMessage(lang Language, code int32) string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, provider := range p.providers {
		if msg := provider.GetMessage(lang, code); msg != "" {
			return msg
		}
	}
	return ""
}

// SetMessage 设置到第一个源
func (p *MultiSourceTranslationProvider) SetMessage(lang Language, code int32, message string) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.providers) > 0 {
		p.providers[0].SetMessage(lang, code, message)
	}
}

// HasMessage 检查任何源
func (p *MultiSourceTranslationProvider) HasMessage(lang Language, code int32) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, provider := range p.providers {
		if provider.HasMessage(lang, code) {
			return true
		}
	}
	return false
}

// AddProvider 添加新源
func (p *MultiSourceTranslationProvider) AddProvider(provider TranslationProvider) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.providers = append(p.providers, provider)
}
