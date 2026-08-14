package logic

import (
	"context"
	"strings"
	"testing"
	"time"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"
	"appforge/services/core/models"
)

func TestSourceTriggerSecretsAreHighEntropyAndHashed(t *testing.T) {
	one, err := randomSourceTriggerSecret(32)
	if err != nil {
		t.Fatal(err)
	}
	two, err := randomSourceTriggerSecret(32)
	if err != nil {
		t.Fatal(err)
	}
	if one == two || len(one) < 40 || len(sourceTokenHash(one)) != 64 || strings.Contains(sourceTokenHash(one), one) {
		t.Fatal("source trigger token generation or hashing is unsafe")
	}
}

func TestMapSourceBuildTriggerNeverExposesStoredSecrets(t *testing.T) {
	item := &models.TSourceBuildTrigger{Id: 7, TenantId: 9, RepositoryId: 11, AppId: 13,
		TriggerName: "release", EventType: 1, RefPattern: "v*", ArtifactSelector: "app.apk",
		ChannelIds: `[17,19]`, SigningConfigId: 23, PoolCode: "default", WebhookTokenHash: "token-hash",
		WebhookSecretCiphertext: "sb1.secret-ciphertext", Status: 1, CreateTime: time.Now(), UpdateTime: time.Now()}
	mapped := mapSourceBuildTrigger(context.Background(), &svc.ServiceContext{}, item)
	encoded := mapped.String()
	if len(mapped.ChannelIds) != 2 || mapped.ChannelIds[0] != 17 || strings.Contains(encoded, "token-hash") || strings.Contains(encoded, "secret-ciphertext") {
		t.Fatalf("trigger mapping is invalid or leaked secret material: %s", encoded)
	}
}

func TestSourceTriggerInputAndVersionNameNormalization(t *testing.T) {
	name, pattern, selector, pool, err := validateSourceBuildTriggerInput(" Release ",
		core.SourceBuildTriggerEventType_SOURCE_BUILD_TRIGGER_EVENT_TYPE_RELEASE_PUBLISHED, "v[0-9]*", "app.apk", "", "rel-", 2)
	if err != nil || name != "Release" || pattern != "v[0-9]*" || selector != "app.apk" || pool != "default" {
		t.Fatalf("unexpected normalized trigger input: %q %q %q %q err=%v", name, pattern, selector, pool, err)
	}
	if got := normalizedSourceVersionName("rel-", "release/1.2.3"); got != "rel-release-1.2.3" {
		t.Fatalf("unexpected normalized version name: %q", got)
	}
	if _, _, _, _, err := validateSourceBuildTriggerInput("bad", core.SourceBuildTriggerEventType_SOURCE_BUILD_TRIGGER_EVENT_TYPE_RELEASE_PUBLISHED,
		"[", "app.apk", "default", "", 1); err == nil {
		t.Fatal("invalid glob was accepted")
	}
}
