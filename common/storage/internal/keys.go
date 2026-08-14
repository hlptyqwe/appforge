package internal

import (
	"crypto/rand"
	"encoding/hex"
	"path"
	"path/filepath"
	"time"
)

// ObjectInfo describes one object without exposing a storage-provider type.
type ObjectInfo struct {
	Key         string
	Size        int64
	ContentType string
}

// GenerateObjectKey creates a date-based object key suitable for storage.
// This helper is intentionally in an internal package so both the public
// storage package and provider implementations can use the same logic without
// introducing import cycles.
func GenerateObjectKey(filename string) string {
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".bin"
	}

	now := time.Now().UTC()
	yyyy := now.Format("2006")
	mm := now.Format("01")
	dd := now.Format("02")

	return path.Join("uploads", yyyy, mm, dd, randomHex(12)+ext)
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
