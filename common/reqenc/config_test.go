package reqenc

import "testing"

func TestConfigValidation(t *testing.T) {
	disabled := Config{Scope: "app-api", Mode: ModeDisabled}.WithDefaults()
	if err := disabled.Validate(); err != nil {
		t.Fatal(err)
	}
	enabled := Config{Scope: "app-api", Mode: ModeRequired, RSAKid: "key", RSAPrivateKeyPath: "key.pem"}
	if err := enabled.Validate(); err == nil {
		t.Fatal("expected missing wrap key error")
	}
}
