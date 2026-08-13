package generate

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/redis"
)

func GenerateNo(rd *redis.Redis, ctx context.Context, business string, prefix string, bizNo string) (string, error) {
	now := time.Now()
	date := now.Format("20060102")

	// 每天、每个前缀单独计数
	key := fmt.Sprintf("%s:%s:%s", business, prefix, date)

	seq, err := rd.IncrCtx(ctx, key)
	if err != nil {
		return "", err
	}

	// 设置过期时间，避免 Redis 一直堆积旧 key
	// 这里只在第一次创建时设置
	if seq == 1 {
		_ = rd.ExpireCtx(ctx, key, 36*int(time.Hour.Seconds()))
	}

	orderID := fmt.Sprintf("%s%s%06d", prefix, date, seq)
	if bizNo != "" {
		return compactGeneratedNo(fmt.Sprintf("%s_%s", orderID, SanitizeBizNo(bizNo)), 64), nil
	}
	return compactGeneratedNo(orderID, 64), nil
}

func compactGeneratedNo(value string, maxLen int) string {
	if maxLen <= 0 || len(value) <= maxLen {
		return value
	}
	digest := sha256.Sum256([]byte(value))
	hash := fmt.Sprintf("%x", digest[:8])
	headLen := maxLen - len(hash) - 1
	if headLen <= 0 {
		return hash[:maxLen]
	}
	return value[:headLen] + "-" + hash
}

func SanitizeBizNo(bizNo string) string {
	return strings.Map(func(r rune) rune {
		if r == '_' || r == '-' || r == '.' || r == '/' || r == ':' {
			return '_'
		}
		if r >= '0' && r <= '9' {
			return r
		}
		if r >= 'a' && r <= 'z' {
			return r
		}
		if r >= 'A' && r <= 'Z' {
			return r
		}
		return -1
	}, bizNo)
}
