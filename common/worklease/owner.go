package worklease

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"
)

// NewOwner returns a process-local lease owner token suitable for persisted
// claimed_by columns. Randomness prevents two replicas with the same PID from
// sharing an owner identity.
func NewOwner(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "worker"
	}
	var token [12]byte
	if _, err := rand.Read(token[:]); err == nil {
		return fmt.Sprintf("%s-%d-%s", prefix, os.Getpid(), hex.EncodeToString(token[:]))
	}
	return fmt.Sprintf("%s-%d-%d", prefix, os.Getpid(), time.Now().UnixNano())
}
