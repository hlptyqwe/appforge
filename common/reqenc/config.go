package reqenc

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type Mode string

const (
	ModeDisabled Mode = "DISABLED"
	ModeOptional Mode = "OPTIONAL"
	ModeRequired Mode = "REQUIRED"
)

const (
	Version          = "v1"
	KeyAlgorithm     = "RSA-OAEP-256"
	ContentAlgorithm = "A256GCM"
)

type Config struct {
	Scope               string   `json:"Scope" yaml:"Scope"`
	Mode                Mode     `json:"Mode" yaml:"Mode"`
	RSAKid              string   `json:"RSAKid" yaml:"RSAKid"`
	RSAPrivateKeyPath   string   `json:"RSAPrivateKeyPath" yaml:"RSAPrivateKeyPath"`
	SessionWrapKey      string   `json:"SessionWrapKey" yaml:"SessionWrapKey"`
	SessionTTLSeconds   int      `json:"SessionTTLSeconds" yaml:"SessionTTLSeconds"`
	RotateBeforeSeconds int      `json:"RotateBeforeSeconds" yaml:"RotateBeforeSeconds"`
	ClockSkewSeconds    int      `json:"ClockSkewSeconds" yaml:"ClockSkewSeconds"`
	NonceTTLSeconds     int      `json:"NonceTTLSeconds" yaml:"NonceTTLSeconds"`
	MaxPlaintextBytes   int64    `json:"MaxPlaintextBytes" yaml:"MaxPlaintextBytes"`
	MaxCipherTextBytes  int64    `json:"MaxCipherTextBytes" yaml:"MaxCipherTextBytes"`
	ProtectedPrefixes   []string `json:"ProtectedPrefixes" yaml:"ProtectedPrefixes"`
}

func (c Config) WithDefaults() Config {
	c.Scope = strings.TrimSpace(c.Scope)
	c.Mode = Mode(strings.ToUpper(strings.TrimSpace(string(c.Mode))))
	for index, prefix := range c.ProtectedPrefixes {
		c.ProtectedPrefixes[index] = strings.TrimSpace(prefix)
	}
	if c.Mode == "" {
		c.Mode = ModeDisabled
	}
	if c.SessionTTLSeconds <= 0 {
		c.SessionTTLSeconds = 960
	}
	if c.RotateBeforeSeconds <= 0 {
		c.RotateBeforeSeconds = 180
	}
	if c.ClockSkewSeconds <= 0 {
		c.ClockSkewSeconds = 120
	}
	if c.NonceTTLSeconds <= 0 {
		c.NonceTTLSeconds = 120
	}
	if c.MaxPlaintextBytes <= 0 {
		c.MaxPlaintextBytes = 1 << 20
	}
	if c.MaxCipherTextBytes <= 0 {
		c.MaxCipherTextBytes = c.MaxPlaintextBytes + 4096
	}
	return c
}

func (c Config) Validate() error {
	c = c.WithDefaults()
	if c.Mode != ModeDisabled && c.Mode != ModeOptional && c.Mode != ModeRequired {
		return fmt.Errorf("invalid request encryption mode %q", c.Mode)
	}
	if c.Scope == "" {
		return errors.New("request encryption scope is required")
	}
	if c.Mode == ModeDisabled {
		return nil
	}
	if c.RSAKid == "" || c.RSAPrivateKeyPath == "" {
		return errors.New("rsa kid and private key path are required when request encryption is enabled")
	}
	if len([]byte(c.SessionWrapKey)) != 32 {
		return errors.New("session wrap key must be exactly 32 bytes")
	}
	if c.RotateBeforeSeconds >= c.SessionTTLSeconds {
		return errors.New("rotate before seconds must be smaller than session ttl")
	}
	return nil
}

func (c Config) SessionTTL() time.Duration {
	return time.Duration(c.WithDefaults().SessionTTLSeconds) * time.Second
}
func (c Config) RotateBefore() time.Duration {
	return time.Duration(c.WithDefaults().RotateBeforeSeconds) * time.Second
}
func (c Config) ClockSkew() time.Duration {
	return time.Duration(c.WithDefaults().ClockSkewSeconds) * time.Second
}
func (c Config) NonceTTL() time.Duration {
	return time.Duration(c.WithDefaults().NonceTTLSeconds) * time.Second
}
