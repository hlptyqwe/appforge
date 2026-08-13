package utils

import (
	"encoding/base64"
	"errors"
	"net/url"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/skip2/go-qrcode"
)

const (
	Default2FAIssuer = "appforge"
)

func GenerateGoogle2FASecret(account, issuer string) (string, string, error) {
	if account == "" {
		return "", "", errors.New("account is required")
	}
	if issuer == "" {
		issuer = Default2FAIssuer
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: account,
		Digits:      otp.DigitsSix,
		Period:      30,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		return "", "", err
	}

	return key.Secret(), key.URL(), nil
}

func GenerateGoogle2FAQRCodeDataURL(otpauthURL string, size int) (string, error) {
	if otpauthURL == "" {
		return "", nil
	}
	if _, err := url.ParseRequestURI(otpauthURL); err != nil {
		return "", err
	}
	if size <= 0 {
		size = 200
	}

	png, err := qrcode.Encode(otpauthURL, qrcode.Medium, size)
	if err != nil {
		return "", err
	}

	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), nil
}

func VerifyGoogle2FACode(secret, code string) bool {
	if secret == "" || code == "" {
		return false
	}
	return totp.Validate(code, secret)
}

func GenerateGoogle2FA(account, issuer string, qrSize int) (secret, otpauthURL, qrCode string, err error) {
	secret, otpauthURL, err = GenerateGoogle2FASecret(account, issuer)
	if err != nil {
		return "", "", "", err
	}
	qrCode, err = GenerateGoogle2FAQRCodeDataURL(otpauthURL, qrSize)
	if err != nil {
		return "", "", "", err
	}
	return secret, otpauthURL, qrCode, nil
}
