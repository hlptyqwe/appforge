package secretbox

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestSealOpen(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	box, err := New(key)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	sealed, err := box.Seal("keystore-password")
	if err != nil {
		t.Fatalf("Seal() error = %v", err)
	}
	if !IsSealed(sealed) || strings.Contains(sealed, "keystore-password") {
		t.Fatalf("Seal() returned an invalid envelope")
	}
	opened, err := box.Open(sealed)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if opened != "keystore-password" {
		t.Fatalf("Open() = %q", opened)
	}
}

func TestOpenRejectsTampering(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	box, _ := New(key)
	sealed, _ := box.Seal("secret")
	last := sealed[len(sealed)-1]
	replacement := byte('A')
	if last == replacement {
		replacement = 'B'
	}
	sealed = sealed[:len(sealed)-1] + string(replacement)
	if _, err := box.Open(sealed); err == nil {
		t.Fatal("Open() accepted tampered ciphertext")
	}
}

func TestNewRejectsInvalidKey(t *testing.T) {
	if _, err := New("not-a-key"); err == nil {
		t.Fatal("New() accepted an invalid key")
	}
}
