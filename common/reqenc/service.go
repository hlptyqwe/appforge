package reqenc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/hex"
	"errors"
	"time"
)

type Service struct {
	config     Config
	store      Store
	privateKey *rsa.PrivateKey
	publicKey  string
	now        func() time.Time
}

func New(config Config, store Store) (*Service, error) {
	config = config.WithDefaults()
	if err := config.Validate(); err != nil {
		return nil, err
	}
	service := &Service{config: config, store: store, now: time.Now}
	if config.Mode == ModeDisabled {
		return service, nil
	}
	if store == nil {
		return nil, errors.New("request encryption store is required")
	}
	privateKey, publicKey, err := LoadRSAPrivateKey(config.RSAPrivateKeyPath)
	if err != nil {
		return nil, err
	}
	service.privateKey, service.publicKey = privateKey, publicKey
	return service, nil
}

func (s *Service) ConfigData() EncryptionConfigData {
	c := s.config
	enabled := c.Mode != ModeDisabled
	return EncryptionConfigData{
		Version: Version, Mode: c.Mode, Enabled: enabled, Required: c.Mode == ModeRequired,
		RSAKid: c.RSAKid, PublicKey: s.publicKey, KeyAlgorithm: KeyAlgorithm,
		ContentAlgorithm: ContentAlgorithm, SessionTTLSeconds: c.SessionTTLSeconds,
		RotateBeforeSeconds: c.RotateBeforeSeconds, ServerTime: s.now().UnixMilli(),
		ProtectedPrefixes: append([]string(nil), c.ProtectedPrefixes...),
	}
}

func (s *Service) CreateSession(ctx context.Context, req CreateSessionRequest) (*CreateSessionData, error) {
	if s.config.Mode == ModeDisabled {
		return nil, ErrDisabled
	}
	if req.Version != Version || req.RSAKid != s.config.RSAKid || req.EncryptedKey == "" {
		return nil, ErrInvalidPayload
	}
	aesKey, err := unwrapClientKey(s.privateKey, req.EncryptedKey)
	if err != nil {
		return nil, err
	}
	wrapped, err := wrapSessionKey([]byte(s.config.SessionWrapKey), aesKey)
	if err != nil {
		return nil, err
	}
	keyIDRaw := make([]byte, 16)
	if _, err := rand.Read(keyIDRaw); err != nil {
		return nil, err
	}
	keyID := hex.EncodeToString(keyIDRaw)
	now := s.now()
	expiresAt := now.Add(s.config.SessionTTL())
	session := Session{
		Version: Version, RSAKid: s.config.RSAKid, WrappedKey: wrapped,
		CreatedAt: now.UnixMilli(), ExpiresAt: expiresAt.UnixMilli(),
	}
	if err := s.store.PutSession(ctx, keyID, session, s.config.SessionTTL()); err != nil {
		return nil, err
	}
	return &CreateSessionData{
		KeyID: keyID, ExpiresAt: expiresAt.UnixMilli(),
		RotateAfter: expiresAt.Add(-s.config.RotateBefore()).UnixMilli(),
	}, nil
}
