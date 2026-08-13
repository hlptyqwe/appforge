package reqenc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/redis"
)

type Store interface {
	PutSession(context.Context, string, Session, time.Duration) error
	GetSession(context.Context, string) (*Session, error)
	UseNonce(context.Context, string, string, time.Duration) (bool, error)
	DeleteSession(context.Context, string) error
}

type RedisStore struct {
	redis *redis.Redis
	scope string
}

func NewRedisStore(rds *redis.Redis, scope string) *RedisStore {
	return &RedisStore{redis: rds, scope: scope}
}

func (s *RedisStore) sessionKey(keyID string) string {
	return fmt.Sprintf("security:encryption:%s:session:%s", s.scope, keyID)
}

func (s *RedisStore) nonceKey(keyID string, nonce string) string {
	return fmt.Sprintf("security:encryption:%s:nonce:%s:%s", s.scope, keyID, nonce)
}

func (s *RedisStore) PutSession(ctx context.Context, keyID string, session Session, ttl time.Duration) error {
	raw, err := json.Marshal(session)
	if err != nil {
		return err
	}
	return s.redis.SetexCtx(ctx, s.sessionKey(keyID), string(raw), max(1, int(ttl.Seconds())))
}

func (s *RedisStore) GetSession(ctx context.Context, keyID string) (*Session, error) {
	raw, err := s.redis.GetCtx(ctx, s.sessionKey(keyID))
	if err != nil || raw == "" {
		return nil, ErrKeyExpired
	}
	var session Session
	if err := json.Unmarshal([]byte(raw), &session); err != nil {
		return nil, ErrInvalidPayload
	}
	return &session, nil
}

func (s *RedisStore) UseNonce(ctx context.Context, keyID string, nonce string, ttl time.Duration) (bool, error) {
	return s.redis.SetnxExCtx(ctx, s.nonceKey(keyID, nonce), "1", max(1, int(ttl.Seconds())))
}

func (s *RedisStore) DeleteSession(ctx context.Context, keyID string) error {
	_, err := s.redis.DelCtx(ctx, s.sessionKey(keyID))
	return err
}

var _ Store = (*RedisStore)(nil)
