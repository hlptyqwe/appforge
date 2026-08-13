package reqenc

import "errors"

type Location string

const (
	LocationJSON  Location = "JSON"
	LocationForm  Location = "FORM"
	LocationQuery Location = "QUERY"
	LocationPath  Location = "PATH"
)

const (
	HeaderVersion   = "X-Encryption-Version"
	HeaderLocation  = "X-Encryption-Location"
	HeaderKeyID     = "X-Encryption-Key-Id"
	HeaderTimestamp = "X-Timestamp"
	HeaderNonce     = "X-Nonce"
)

var (
	ErrDisabled       = errors.New("ENCRYPTION_DISABLED")
	ErrRequired       = errors.New("ENCRYPTION_REQUIRED")
	ErrKeyExpired     = errors.New("ENCRYPTION_KEY_EXPIRED")
	ErrReplayed       = errors.New("ENCRYPTION_REPLAYED")
	ErrInvalidPayload = errors.New("ENCRYPTION_INVALID_PAYLOAD")
)

type EncryptionConfigData struct {
	Version             string   `json:"version"`
	Mode                Mode     `json:"mode"`
	Enabled             bool     `json:"enabled"`
	Required            bool     `json:"required"`
	RSAKid              string   `json:"rsaKid,omitempty"`
	PublicKey           string   `json:"publicKey,omitempty"`
	KeyAlgorithm        string   `json:"keyAlgorithm,omitempty"`
	ContentAlgorithm    string   `json:"contentAlgorithm,omitempty"`
	SessionTTLSeconds   int      `json:"sessionTtlSeconds"`
	RotateBeforeSeconds int      `json:"rotateBeforeSeconds"`
	ServerTime          int64    `json:"serverTime"`
	ProtectedPrefixes   []string `json:"protectedPrefixes"`
}

type CreateSessionRequest struct {
	Version      string `json:"version"`
	RSAKid       string `json:"rsaKid"`
	EncryptedKey string `json:"encryptedKey"`
}

type CreateSessionData struct {
	KeyID       string `json:"keyId"`
	ExpiresAt   int64  `json:"expiresAt"`
	RotateAfter int64  `json:"rotateAfter"`
}

type JSONEnvelope struct {
	IV         string `json:"iv"`
	CipherText string `json:"cipherText"`
}

type Session struct {
	Version    string `json:"version"`
	RSAKid     string `json:"rsaKid"`
	WrappedKey string `json:"wrappedKey"`
	CreatedAt  int64  `json:"createdAt"`
	ExpiresAt  int64  `json:"expiresAt"`
}

type Response struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}
