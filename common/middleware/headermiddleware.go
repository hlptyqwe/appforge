package middleware

import (
	"context"
	"net/http"
	"strconv"
	"appforge/common/utils"
)

type HeaderMiddleware struct{}

func NewHeaderMiddleware() *HeaderMiddleware {
	return &HeaderMiddleware{}
}

func (m *HeaderMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		if uidStr := r.Header.Get(string(utils.CtxKeyUid)); uidStr != "" {
			if userId, err := strconv.ParseInt(uidStr, 10, 64); err == nil {
				ctx = context.WithValue(ctx, utils.CtxKeyUid, userId)
			}
		}

		if username := r.Header.Get(string(utils.CtxKeyUsername)); username != "" {
			ctx = context.WithValue(ctx, utils.CtxKeyUsername, username)
		}

		if tenantIdStr := r.Header.Get(string(utils.CtxKeyTenantId)); tenantIdStr != "" {
			if tenantId, err := strconv.ParseInt(tenantIdStr, 10, 64); err == nil {
				ctx = context.WithValue(ctx, utils.CtxKeyTenantId, tenantId)
			}
		}

		if tenantCode := r.Header.Get(string(utils.CtxKeyTenantCode)); tenantCode != "" {
			ctx = context.WithValue(ctx, utils.CtxKeyTenantCode, tenantCode)
		}

		r = r.WithContext(ctx)
		next(w, r)
	}
}
