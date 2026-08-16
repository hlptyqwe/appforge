package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/redis"
)

var errAgentArtifactStateNotFound = errors.New("Local Agent Artifact state not found")

type agentArtifactRegistry interface {
	PutTicket(context.Context, string, agentArtifactTicket, time.Duration) error
	ConsumeTicket(context.Context, string) (*agentArtifactTicket, error)
	PutTask(context.Context, string, agentArtifactTask, time.Duration) error
	GetTask(context.Context, string) (*agentArtifactTask, error)
	PutUploadIfAbsent(context.Context, string, agentUploadedArtifact, time.Duration) (bool, error)
	GetUpload(context.Context, string) (*agentUploadedArtifact, error)
	Delete(context.Context, ...string) error
}

type redisAgentArtifactRegistry struct {
	client *redis.Redis
	prefix string
}

func newRedisAgentArtifactRegistry(client *redis.Redis) *redisAgentArtifactRegistry {
	return &redisAgentArtifactRegistry{client: client, prefix: "appforge:local-agent:artifact:"}
}

func (r *redisAgentArtifactRegistry) PutTicket(ctx context.Context, token string, ticket agentArtifactTicket, ttl time.Duration) error {
	return r.put(ctx, r.ticketKey(token), ticket, ttl)
}

func (r *redisAgentArtifactRegistry) ConsumeTicket(ctx context.Context, token string) (*agentArtifactTicket, error) {
	raw, err := r.client.GetDelCtx(ctx, r.ticketKey(token))
	if err != nil {
		return nil, err
	}
	if raw == "" {
		return nil, errAgentArtifactStateNotFound
	}
	var ticket agentArtifactTicket
	if err := json.Unmarshal([]byte(raw), &ticket); err != nil {
		return nil, fmt.Errorf("decode Local Agent Artifact ticket: %w", err)
	}
	return &ticket, nil
}

func (r *redisAgentArtifactRegistry) PutTask(ctx context.Context, key string, task agentArtifactTask, ttl time.Duration) error {
	return r.put(ctx, r.taskKey(key), task, ttl)
}

func (r *redisAgentArtifactRegistry) GetTask(ctx context.Context, key string) (*agentArtifactTask, error) {
	var task agentArtifactTask
	if err := r.get(ctx, r.taskKey(key), &task); err != nil {
		return nil, err
	}
	return &task, nil
}

func (r *redisAgentArtifactRegistry) PutUploadIfAbsent(ctx context.Context, key string, upload agentUploadedArtifact, ttl time.Duration) (bool, error) {
	raw, err := json.Marshal(upload)
	if err != nil {
		return false, err
	}
	return r.client.SetnxExCtx(ctx, r.uploadKey(key), string(raw), max(1, int(ttl.Seconds())))
}

func (r *redisAgentArtifactRegistry) GetUpload(ctx context.Context, key string) (*agentUploadedArtifact, error) {
	var upload agentUploadedArtifact
	if err := r.get(ctx, r.uploadKey(key), &upload); err != nil {
		return nil, err
	}
	return &upload, nil
}

func (r *redisAgentArtifactRegistry) Delete(ctx context.Context, keys ...string) error {
	resolved := make([]string, 0, len(keys))
	for _, key := range keys {
		resolved = append(resolved, r.prefix+key)
	}
	_, err := r.client.DelCtx(ctx, resolved...)
	return err
}

func (r *redisAgentArtifactRegistry) put(ctx context.Context, key string, value any, ttl time.Duration) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return r.client.SetexCtx(ctx, key, string(raw), max(1, int(ttl.Seconds())))
}

func (r *redisAgentArtifactRegistry) get(ctx context.Context, key string, target any) error {
	raw, err := r.client.GetCtx(ctx, key)
	if err != nil {
		return err
	}
	if raw == "" {
		return errAgentArtifactStateNotFound
	}
	if err := json.Unmarshal([]byte(raw), target); err != nil {
		return fmt.Errorf("decode Local Agent Artifact state: %w", err)
	}
	return nil
}

func (r *redisAgentArtifactRegistry) ticketKey(token string) string {
	digest := sha256.Sum256([]byte(token))
	return r.prefix + "ticket:" + hex.EncodeToString(digest[:])
}

func (r *redisAgentArtifactRegistry) taskKey(key string) string   { return r.prefix + "task:" + key }
func (r *redisAgentArtifactRegistry) uploadKey(key string) string { return r.prefix + "upload:" + key }
