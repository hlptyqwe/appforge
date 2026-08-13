package utils

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"

	"google.golang.org/grpc/metadata"
)

func GetClientIP(r *http.Request) string {
	// 1. X-Forwarded-For（可能多个）
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		ips := strings.Split(ip, ",")
		return strings.TrimSpace(ips[0])
	}

	// 2. X-Real-IP
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}

	// 3. RemoteAddr
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}

func ContextWithClientIP(ctx context.Context, ip string) context.Context {
	if ip == "" {
		return ctx
	}
	return context.WithValue(ctx, CtxKeyClientIp, ip)
}

func GetClientIPFromCtx(ctx context.Context) (string, error) {
	ip, ok := ctx.Value(CtxKeyClientIp).(string)
	if !ok || ip == "" {
		return "", fmt.Errorf("%s not found in context", CtxKeyClientIp)
	}
	return ip, nil
}

func GetClientIPFromMd(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", fmt.Errorf("no metadata in context")
	}

	if vals := md.Get(CtxKeyClientIp); len(vals) > 0 && vals[0] != "" {
		return vals[0], nil
	}

	return "", fmt.Errorf("%s not found in metadata", CtxKeyClientIp)
}

func GetUserInfoFromMd(ctx context.Context) (int64, string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return 0, "", fmt.Errorf("no metadata in context")
	}

	var userId int64
	if vals := md.Get(CtxKeyUid); len(vals) > 0 && vals[0] != "" {
		var err error
		userId, err = strconv.ParseInt(vals[0], 10, 64)
		if err != nil {
			return 0, "", fmt.Errorf("invalid x-user-id: %w", err)
		}
	}

	var username string
	if vals := md.Get(CtxKeyUsername); len(vals) > 0 {
		username = vals[0]
	}

	return userId, username, nil
}

func GetUserIdFromMd(ctx context.Context) (int64, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return 0, fmt.Errorf("no metadata in context")
	}

	var userId int64
	if vals := md.Get(CtxKeyUid); len(vals) > 0 && vals[0] != "" {
		var err error
		userId, err = strconv.ParseInt(vals[0], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid x-user-id: %w", err)
		}
	}

	return userId, nil
}

func GetUsernameFromMd(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", fmt.Errorf("no metadata in context")
	}

	var username string
	if vals := md.Get(CtxKeyUsername); len(vals) > 0 {
		username = vals[0]
	}

	return username, nil
}

func GetTenantIdFromMd(ctx context.Context) (int64, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return 0, fmt.Errorf("no metadata in context")
	}

	var tenantId int64
	if vals := md.Get(CtxKeyTenantId); len(vals) > 0 && vals[0] != "" {
		var err error
		tenantId, err = strconv.ParseInt(vals[0], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid x-tenant-id: %w", err)
		}
	}

	return tenantId, nil
}

func GetMerchantIdFromMd(ctx context.Context) (int64, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return 0, fmt.Errorf("no metadata in context")
	}

	var merchantId int64
	if vals := md.Get(CtxKeyMerchantId); len(vals) > 0 && vals[0] != "" {
		var err error
		merchantId, err = strconv.ParseInt(vals[0], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid x-merchant-id: %w", err)
		}
	}

	return merchantId, nil
}

func GetUserTypeFromMd(ctx context.Context) (int64, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return 0, fmt.Errorf("no metadata in context")
	}

	var userType int64
	if vals := md.Get(CtxKeyUserType); len(vals) > 0 && vals[0] != "" {
		var err error
		userType, err = strconv.ParseInt(vals[0], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid x-user-type: %w", err)
		}
	}

	return userType, nil
}

func GetTenantCodeFromMd(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", fmt.Errorf("no metadata in context")
	}

	var tenantCode string
	if vals := md.Get(CtxKeyTenantCode); len(vals) > 0 {
		tenantCode = vals[0]
	}

	return tenantCode, nil
}

func GetSubjectDomainFromMd(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", fmt.Errorf("no metadata in context")
	}
	if vals := md.Get(CtxKeySubjectDomain); len(vals) > 0 && vals[0] != "" {
		return vals[0], nil
	}
	return "", fmt.Errorf("%s not found in metadata", CtxKeySubjectDomain)
}
