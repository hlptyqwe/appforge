package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
)

type RequestLogMiddleware struct {
	service string
}

func NewRequestLogMiddleware(service string) *RequestLogMiddleware {
	return &RequestLogMiddleware{service: strings.TrimSpace(service)}
}

func (m *RequestLogMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		if isWebsocketRequest(r) {
			logx.WithContext(r.Context()).Infof(
				"[%s WS CONNECTED] method=%s uri=%s host=%s remote=%s origin=%s ua=%s",
				m.serviceName(),
				r.Method,
				r.RequestURI,
				r.Host,
				r.RemoteAddr,
				r.Header.Get("Origin"),
				r.UserAgent(),
			)
			next(w, r)
			logx.WithContext(r.Context()).Infof(
				"[%s WS CLOSED] method=%s uri=%s duration=%s",
				m.serviceName(),
				r.Method,
				r.RequestURI,
				time.Since(start),
			)
			return
		}

		logx.WithContext(r.Context()).Infof(
			"[%s HTTP REQUEST] method=%s uri=%s host=%s remote=%s origin=%s ua=%s",
			m.serviceName(),
			r.Method,
			r.RequestURI,
			r.Host,
			r.RemoteAddr,
			r.Header.Get("Origin"),
			r.UserAgent(),
		)
		next(w, r)
		logx.WithContext(r.Context()).Infof(
			"[%s HTTP DONE] method=%s uri=%s duration=%s",
			m.serviceName(),
			r.Method,
			r.RequestURI,
			time.Since(start),
		)
	}
}

func (m *RequestLogMiddleware) serviceName() string {
	if m.service != "" {
		return m.service
	}
	return "API"
}

func isWebsocketRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		return false
	}
	for _, part := range strings.Split(r.Header.Get("Connection"), ",") {
		if strings.EqualFold(strings.TrimSpace(part), "upgrade") {
			return true
		}
	}
	return false
}
