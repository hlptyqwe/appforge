// Package secretbox provides versioned authenticated encryption for secrets
// that must be persisted and later consumed by trusted AppForge services.
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const version = "sb1"

var (
	// ErrInvalidKey indicates that the configured master key is not a base64
	// encoded 32-byte AES-256 key.
	ErrInvalidKey = errors.New("secretbox key must be base64 encoded 32 bytes")
	// ErrInvalidCiphertext indicates malformed, unsupported or unauthenticated
	// encrypted data.
	ErrInvalidCiphertext = errors.New("invalid secretbox ciphertext")
)

// Box encrypts and decrypts persisted application secrets with AES-256-GCM.
type Box struct {
	aead cipher.AEAD
}

// New creates a Box from a base64 encoded 32-byte key.
func New(keyBase64 string) (*Box, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(keyBase64))
	if err != nil || len(key) != 32 {
		return nil, ErrInvalidKey
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM cipher: %w", err)
	}
	return &Box{aead: aead}, nil
}

// Seal encrypts plaintext and returns a versioned, URL-safe value.
func (b *Box) Seal(plaintext string) (string, error) {
	if b == nil || b.aead == nil {
		return "", ErrInvalidKey
	}
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	ciphertext := b.aead.Seal(nil, nonce, []byte(plaintext), []byte(version))
	return strings.Join([]string{
		version,
		base64.RawURLEncoding.EncodeToString(nonce),
		base64.RawURLEncoding.EncodeToString(ciphertext),
	}, "."), nil
}

// Open authenticates and decrypts a value returned by Seal.
func (b *Box) Open(value string) (string, error) {
	if b == nil || b.aead == nil {
		return "", ErrInvalidKey
	}
	parts := strings.Split(value, ".")
	if len(parts) != 3 || parts[0] != version {
		return "", ErrInvalidCiphertext
	}
	nonce, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || len(nonce) != b.aead.NonceSize() {
		return "", ErrInvalidCiphertext
	}
	ciphertext, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", ErrInvalidCiphertext
	}
	plaintext, err := b.aead.Open(nil, nonce, ciphertext, []byte(version))
	if err != nil {
		return "", ErrInvalidCiphertext
	}
	return string(plaintext), nil
}

// IsSealed reports whether a value uses the supported envelope format. It
// does not authenticate the value; Open must be used before trusting it.
func IsSealed(value string) bool {
	return strings.HasPrefix(strings.TrimSpace(value), version+".")
}
