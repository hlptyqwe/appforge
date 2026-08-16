package logic

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"appforge/proto/core"
	"appforge/services/core/models"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestBuilderNodeSupportsRemoteSigning(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		capability sql.NullString
		want       bool
	}{
		{name: "enabled", capability: sql.NullString{String: `{"apk":true,"remoteSigning":true}`, Valid: true}, want: true},
		{name: "disabled", capability: sql.NullString{String: `{"remoteSigning":false}`, Valid: true}},
		{name: "missing", capability: sql.NullString{String: `{"apk":true}`, Valid: true}},
		{name: "wrong type", capability: sql.NullString{String: `{"remoteSigning":"true"}`, Valid: true}},
		{name: "invalid", capability: sql.NullString{String: `{`, Valid: true}},
		{name: "null"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			node := &models.TBuilderNode{CapabilityJson: tt.capability}
			if got := builderNodeSupportsRemoteSigning(node); got != tt.want {
				t.Fatalf("builderNodeSupportsRemoteSigning() = %t, want %t", got, tt.want)
			}
		})
	}
	if builderNodeSupportsRemoteSigning(nil) {
		t.Fatal("nil builder node must not support remote signing")
	}
}

func TestEffectiveBuildCacheKey(t *testing.T) {
	t.Parallel()

	inputDigest := strings.Repeat("a", 64)
	base := effectiveBuildCacheKey(inputDigest, "android-debian-v4", 1)
	if !validSHA256(base) {
		t.Fatalf("effectiveBuildCacheKey() = %q, want lowercase SHA-256", base)
	}
	if again := effectiveBuildCacheKey(inputDigest, "android-debian-v4", 1); again != base {
		t.Fatalf("effectiveBuildCacheKey() is not deterministic: first=%q second=%q", base, again)
	}

	tests := []struct {
		name             string
		inputDigest      string
		toolchainVersion string
		protocolVersion  int32
	}{
		{name: "input digest", inputDigest: strings.Repeat("b", 64), toolchainVersion: "android-debian-v4", protocolVersion: 1},
		{name: "toolchain", inputDigest: inputDigest, toolchainVersion: "android-debian-v5", protocolVersion: 1},
		{name: "protocol", inputDigest: inputDigest, toolchainVersion: "android-debian-v4", protocolVersion: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := effectiveBuildCacheKey(tt.inputDigest, tt.toolchainVersion, tt.protocolVersion)
			if got == base {
				t.Fatalf("effectiveBuildCacheKey() did not change when %s changed", tt.name)
			}
		})
	}
}

func TestBuilderNodeRecoveryError(t *testing.T) {
	t.Parallel()
	now := time.Now()
	healthy := models.TBuilderNode{
		Status:              int64(core.BuilderNodeStatus_BUILDER_NODE_STATUS_ISOLATED),
		DiskCapacity:        100 * 1024 * 1024 * 1024,
		DiskFree:            10 * 1024 * 1024 * 1024,
		LastHeartbeatAt:     now.Add(-time.Second),
		ConsecutiveFailures: 0,
	}
	if err := builderNodeRecoveryError(&healthy, now); err != nil {
		t.Fatalf("builderNodeRecoveryError() healthy node error = %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*models.TBuilderNode)
	}{
		{name: "not isolated", mutate: func(node *models.TBuilderNode) { node.Status = builderNodeStatusOnline }},
		{name: "stale heartbeat", mutate: func(node *models.TBuilderNode) { node.LastHeartbeatAt = now.Add(-2 * time.Minute) }},
		{name: "future heartbeat", mutate: func(node *models.TBuilderNode) { node.LastHeartbeatAt = now.Add(time.Second) }},
		{name: "still failing", mutate: func(node *models.TBuilderNode) { node.ConsecutiveFailures = 1 }},
		{name: "less than 512 MiB", mutate: func(node *models.TBuilderNode) { node.DiskFree = builderMinimumDiskFree - 1 }},
		{name: "less than two percent", mutate: func(node *models.TBuilderNode) {
			node.DiskFree = 1024 * 1024 * 1024
			node.DiskCapacity = 100 * 1024 * 1024 * 1024
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := healthy
			tt.mutate(&node)
			if status.Code(builderNodeRecoveryError(&node, now)) != codes.FailedPrecondition {
				t.Fatal("builderNodeRecoveryError() must reject unsafe recovery")
			}
		})
	}
}

func TestNormalizedBuildPool(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr codes.Code
	}{
		{name: "default when empty", input: "", want: defaultBuildPool, wantErr: codes.OK},
		{name: "trim valid pool", input: "  android_prod  ", want: "android_prod", wantErr: codes.OK},
		{name: "reject one character", input: "a", wantErr: codes.InvalidArgument},
		{name: "reject uppercase", input: "Android", wantErr: codes.InvalidArgument},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizedBuildPool(tt.input)
			if status.Code(err) != tt.wantErr {
				t.Fatalf("normalizedBuildPool() code = %v, want %v", status.Code(err), tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("normalizedBuildPool() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizedBuilderNodeCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr codes.Code
	}{
		{name: "trim valid code", input: "  builder-dev-01  ", want: "builder-dev-01", wantErr: codes.OK},
		{name: "reject empty", input: "", wantErr: codes.InvalidArgument},
		{name: "reject whitespace", input: "  ", wantErr: codes.InvalidArgument},
		{name: "reject uppercase", input: "Builder-01", wantErr: codes.InvalidArgument},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizedBuilderNodeCode(tt.input)
			if status.Code(err) != tt.wantErr {
				t.Fatalf("normalizedBuilderNodeCode() code = %v, want %v", status.Code(err), tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("normalizedBuilderNodeCode() = %q, want %q", got, tt.want)
			}
		})
	}
}
