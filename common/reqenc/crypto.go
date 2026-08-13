package reqenc

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
)

var base64URL = base64.RawURLEncoding

func LoadRSAPrivateKey(path string) (*rsa.PrivateKey, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, "", errors.New("invalid rsa private key pem")
	}
	var key *rsa.PrivateKey
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err == nil {
		var ok bool
		key, ok = parsed.(*rsa.PrivateKey)
		if !ok {
			return nil, "", errors.New("private key is not rsa")
		}
	} else {
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, "", fmt.Errorf("parse rsa private key: %w", err)
		}
	}
	if key.N.BitLen() < 2048 {
		return nil, "", errors.New("rsa private key must be at least 2048 bits")
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, "", err
	}
	return key, base64.StdEncoding.EncodeToString(publicDER), nil
}

func unwrapClientKey(privateKey *rsa.PrivateKey, encoded string) ([]byte, error) {
	cipherText, err := base64URL.DecodeString(encoded)
	if err != nil {
		return nil, ErrInvalidPayload
	}
	key, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, privateKey, cipherText, nil)
	if err != nil || len(key) != 32 {
		return nil, ErrInvalidPayload
	}
	return key, nil
}

func encryptGCM(key []byte, plaintext []byte, aad []byte) (nonce []byte, cipherText []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce = make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	return nonce, gcm.Seal(nil, nonce, plaintext, aad), nil
}

func decryptGCM(key []byte, nonce []byte, cipherText []byte, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrInvalidPayload
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil || len(nonce) != gcm.NonceSize() {
		return nil, ErrInvalidPayload
	}
	plaintext, err := gcm.Open(nil, nonce, cipherText, aad)
	if err != nil {
		return nil, ErrInvalidPayload
	}
	return plaintext, nil
}

func wrapSessionKey(wrapKey []byte, sessionKey []byte) (string, error) {
	nonce, cipherText, err := encryptGCM(wrapKey, sessionKey, nil)
	if err != nil {
		return "", err
	}
	return base64URL.EncodeToString(append(nonce, cipherText...)), nil
}

func unwrapSessionKey(wrapKey []byte, encoded string) ([]byte, error) {
	raw, err := base64URL.DecodeString(encoded)
	if err != nil || len(raw) < 12+16 {
		return nil, ErrInvalidPayload
	}
	return decryptGCM(wrapKey, raw[:12], raw[12:], nil)
}
