package middleware

import (
	"net/http"
	"sync"
	"time"

	"appforge/common/utils"

	"golang.org/x/time/rate"
)

type publicVisitorLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// PublicRateLimitMiddleware 对公开下载和客户端归因接口按来源IP限流。
type PublicRateLimitMiddleware struct {
	mu       sync.Mutex
	visitors map[string]*publicVisitorLimiter
}

func NewPublicRateLimitMiddleware() *PublicRateLimitMiddleware {
	return &PublicRateLimitMiddleware{visitors: make(map[string]*publicVisitorLimiter)}
}

func (m *PublicRateLimitMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isPublicTraffic(r.URL.Path) {
			next(w, r)
			return
		}
		if !m.allow(utils.GetClientIP(r)) {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

func (m *PublicRateLimitMiddleware) allow(key string) bool {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	visitor := m.visitors[key]
	if visitor == nil {
		visitor = &publicVisitorLimiter{limiter: rate.NewLimiter(5, 15)}
		m.visitors[key] = visitor
	}
	visitor.lastSeen = now
	if len(m.visitors) > 10000 {
		for visitorKey, item := range m.visitors {
			if now.Sub(item.lastSeen) > 10*time.Minute {
				delete(m.visitors, visitorKey)
			}
		}
	}
	return visitor.limiter.Allow()
}

func isPublicTraffic(path string) bool {
	return path == "/api/install/report" || path == "/api/channel/event" || path == "/d" || len(path) > 3 && path[:3] == "/d/"
}
