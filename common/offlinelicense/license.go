// Package offlinelicense verifies vendor-signed AppForge enterprise licenses.
// Runtime components only receive the Ed25519 public key; the issuer private key
// never needs to enter a customer environment.
package offlinelicense

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"
)

// Payload is the canonical signed license body. Timestamps are Unix
// milliseconds so verification does not depend on locale or timezone.
type Payload struct {
	LicenseID       string   `json:"licenseId"`
	Customer        string   `json:"customer"`
	DeploymentID    string   `json:"deploymentId"`
	DeploymentModes []string `json:"deploymentModes"`
	Features        []string `json:"features,omitempty"`
	NotBefore       int64    `json:"notBefore"`
	NotAfter        int64    `json:"notAfter"`
	IssuedAt        int64    `json:"issuedAt"`
	Sequence        uint64   `json:"sequence"`
	MaxTenants      int64    `json:"maxTenants,omitempty"`
	MaxBuilders     int64    `json:"maxBuilders,omitempty"`
}

type Envelope struct {
	Payload   Payload `json:"payload"`
	Signature string  `json:"signature"`
}

type Config struct {
	LicenseFile            string
	PublicKeyFile          string
	StateFile              string
	DeploymentID           string
	DeploymentMode         string
	ClockRollbackTolerance time.Duration
}

type VerifiedLicense struct {
	Payload     Payload `json:"payload"`
	Fingerprint string  `json:"fingerprint"`
}

type verifierState struct {
	LastSeenAt      int64  `json:"lastSeenAt"`
	LastSequence    uint64 `json:"lastSequence"`
	LastLicenseID   string `json:"lastLicenseId"`
	LastFingerprint string `json:"lastFingerprint"`
}

func Sign(payload Payload, privateKey ed25519.PrivateKey) (Envelope, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return Envelope{}, errors.New("Ed25519 private key is invalid")
	}
	if err := validatePayload(payload); err != nil {
		return Envelope{}, err
	}
	canonical, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{Payload: payload, Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, canonical))}, nil
}

func VerifyFile(config Config, now time.Time) (*VerifiedLicense, error) {
	if strings.TrimSpace(config.LicenseFile) == "" || strings.TrimSpace(config.PublicKeyFile) == "" || strings.TrimSpace(config.StateFile) == "" {
		return nil, errors.New("license file, public key file and persistent state file are required")
	}
	raw, err := os.ReadFile(config.LicenseFile)
	if err != nil {
		return nil, fmt.Errorf("read offline license: %w", err)
	}
	var envelope Envelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decode offline license: %w", err)
	}
	publicKey, err := LoadPublicKey(config.PublicKeyFile)
	if err != nil {
		return nil, err
	}
	canonical, err := json.Marshal(envelope.Payload)
	if err != nil {
		return nil, err
	}
	signature, err := base64.StdEncoding.DecodeString(envelope.Signature)
	if err != nil || !ed25519.Verify(publicKey, canonical, signature) {
		return nil, errors.New("offline license signature is invalid")
	}
	if err := validatePayload(envelope.Payload); err != nil {
		return nil, err
	}
	if envelope.Payload.DeploymentID != strings.TrimSpace(config.DeploymentID) {
		return nil, errors.New("offline license is bound to a different deployment")
	}
	if !slices.Contains(envelope.Payload.DeploymentModes, strings.TrimSpace(config.DeploymentMode)) {
		return nil, errors.New("offline license does not allow this deployment mode")
	}
	if err := validAt(envelope.Payload, now); err != nil {
		return nil, err
	}
	fingerprintBytes := sha256.Sum256(raw)
	fingerprint := hex.EncodeToString(fingerprintBytes[:])
	if err := updateVerifierState(config, envelope.Payload, fingerprint, now); err != nil {
		return nil, err
	}
	return &VerifiedLicense{Payload: envelope.Payload, Fingerprint: fingerprint}, nil
}

func (license *VerifiedLicense) ValidAt(now time.Time) error {
	if license == nil {
		return nil
	}
	return validAt(license.Payload, now)
}

func validatePayload(payload Payload) error {
	if strings.TrimSpace(payload.LicenseID) == "" || strings.TrimSpace(payload.Customer) == "" || strings.TrimSpace(payload.DeploymentID) == "" {
		return errors.New("license ID, customer and deployment ID are required")
	}
	if len(payload.DeploymentModes) == 0 || payload.Sequence == 0 {
		return errors.New("deployment modes and positive sequence are required")
	}
	for _, mode := range payload.DeploymentModes {
		switch mode {
		case "dedicated", "private", "hybrid":
		default:
			return fmt.Errorf("unsupported deployment mode %q", mode)
		}
	}
	if payload.NotBefore <= 0 || payload.NotAfter <= payload.NotBefore || payload.IssuedAt <= 0 || payload.IssuedAt > payload.NotAfter {
		return errors.New("license validity timestamps are invalid")
	}
	return nil
}

func validAt(payload Payload, now time.Time) error {
	nowMillis := now.UnixMilli()
	if nowMillis < payload.NotBefore {
		return errors.New("offline license is not active yet")
	}
	if nowMillis >= payload.NotAfter {
		return errors.New("offline license has expired")
	}
	return nil
}

func updateVerifierState(config Config, payload Payload, fingerprint string, now time.Time) error {
	if err := os.MkdirAll(filepath.Dir(config.StateFile), 0o700); err != nil {
		return fmt.Errorf("create license state directory: %w", err)
	}
	lockFile, err := os.OpenFile(config.StateFile+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("open license state lock: %w", err)
	}
	defer lockFile.Close()
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("lock license state: %w", err)
	}
	defer syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN) //nolint:errcheck

	var previous verifierState
	if raw, readErr := os.ReadFile(config.StateFile); readErr == nil {
		if err := json.Unmarshal(raw, &previous); err != nil {
			return errors.New("persistent license state is corrupt")
		}
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return fmt.Errorf("read persistent license state: %w", readErr)
	}
	tolerance := config.ClockRollbackTolerance
	if tolerance <= 0 {
		tolerance = 5 * time.Minute
	}
	if previous.LastSeenAt > 0 && now.Add(tolerance).UnixMilli() < previous.LastSeenAt {
		return errors.New("system clock rollback detected by persistent license state")
	}
	if previous.LastSequence > payload.Sequence {
		return errors.New("offline license rollback detected")
	}
	current := verifierState{LastSeenAt: max(previous.LastSeenAt, now.UnixMilli()), LastSequence: payload.Sequence,
		LastLicenseID: payload.LicenseID, LastFingerprint: fingerprint}
	encoded, err := json.Marshal(current)
	if err != nil {
		return err
	}
	temporary := config.StateFile + ".tmp"
	if err := os.WriteFile(temporary, encoded, 0o600); err != nil {
		return fmt.Errorf("write license state: %w", err)
	}
	if err := os.Rename(temporary, config.StateFile); err != nil {
		return fmt.Errorf("commit license state: %w", err)
	}
	return nil
}

func LoadPublicKey(path string) (ed25519.PublicKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read license public key: %w", err)
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		decoded, decodeErr := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
		if decodeErr == nil && len(decoded) == ed25519.PublicKeySize {
			return ed25519.PublicKey(decoded), nil
		}
		return nil, errors.New("license public key must be PKIX PEM or Base64 Ed25519")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse license public key: %w", err)
	}
	publicKey, ok := parsed.(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("license public key is not Ed25519")
	}
	return publicKey, nil
}
