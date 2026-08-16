package agentpki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"time"

	"appforge/common/airgap"
)

type Certificate struct {
	Serial      string
	Fingerprint string
	PEM         string
	NotBefore   time.Time
	NotAfter    time.Time
}

type Signer struct {
	certificate *x509.Certificate
	privateKey  *ecdsa.PrivateKey
	caPEM       string
	validity    time.Duration
}

func New(certFile, keyFile string, validity time.Duration) (*Signer, error) {
	if validity <= 0 {
		validity = 24 * time.Hour
	}
	if certFile == "" && keyFile == "" {
		return generateEphemeral(validity)
	}
	if certFile == "" || keyFile == "" {
		return nil, errors.New("both AgentPKI CA certificate and private key files are required")
	}
	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		return nil, fmt.Errorf("read Agent CA certificate: %w", err)
	}
	keyPEM, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("read Agent CA private key: %w", err)
	}
	certBlock, _ := pem.Decode(certPEM)
	keyBlock, _ := pem.Decode(keyPEM)
	if certBlock == nil || keyBlock == nil {
		return nil, errors.New("Agent CA PEM is invalid")
	}
	certificate, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil || !certificate.IsCA {
		return nil, errors.New("Agent CA certificate is invalid or is not a CA")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	if err != nil {
		if ecKey, ecErr := x509.ParseECPrivateKey(keyBlock.Bytes); ecErr == nil {
			parsed = ecKey
		} else {
			return nil, fmt.Errorf("parse Agent CA private key: %w", err)
		}
	}
	privateKey, ok := parsed.(*ecdsa.PrivateKey)
	if !ok || !privateKey.PublicKey.Equal(certificate.PublicKey) {
		return nil, errors.New("Agent CA requires a matching ECDSA private key")
	}
	return &Signer{certificate: certificate, privateKey: privateKey, caPEM: string(certPEM), validity: validity}, nil
}

func generateEphemeral(validity time.Duration) (*Signer, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	template := &x509.Certificate{
		SerialNumber: randomSerial(), Subject: pkix.Name{CommonName: "AppForge development Local Agent CA"},
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(30 * 24 * time.Hour), IsCA: true,
		BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	return &Signer{certificate: certificate, privateKey: key,
		caPEM: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), validity: validity}, nil
}

func (s *Signer) SignCSR(csrPEM string, tenantID, agentID int64) (*Certificate, error) {
	if tenantID <= 0 || agentID <= 0 {
		return nil, errors.New("tenant and Agent identities must be positive")
	}
	block, _ := pem.Decode([]byte(csrPEM))
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		return nil, errors.New("CSR PEM is invalid")
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil || csr.CheckSignature() != nil {
		return nil, errors.New("CSR signature is invalid")
	}
	switch csr.PublicKey.(type) {
	case *ecdsa.PublicKey:
	default:
		return nil, errors.New("CSR key must use ECDSA")
	}
	identity, _ := url.Parse(fmt.Sprintf("spiffe://appforge/tenant/%d/local-agent/%d", tenantID, agentID))
	now := time.Now().UTC()
	notAfter := now.Add(s.validity)
	if notAfter.After(s.certificate.NotAfter) {
		notAfter = s.certificate.NotAfter
	}
	template := &x509.Certificate{
		SerialNumber: randomSerial(), Subject: pkix.Name{CommonName: fmt.Sprintf("local-agent-%d", agentID)},
		URIs: []*url.URL{identity}, NotBefore: now.Add(-time.Minute), NotAfter: notAfter,
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, s.certificate, csr.PublicKey, s.privateKey)
	if err != nil {
		return nil, fmt.Errorf("sign Agent certificate: %w", err)
	}
	digest := sha256.Sum256(der)
	return &Certificate{Serial: template.SerialNumber.Text(16), Fingerprint: hex.EncodeToString(digest[:]),
		PEM:       string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})),
		NotBefore: template.NotBefore, NotAfter: template.NotAfter}, nil
}

func (s *Signer) CAPEM() string { return s.caPEM }

// SignManifest signs canonical AIR_GAPPED metadata with the Agent CA key.
func (s *Signer) SignManifest(canonical []byte) (airgap.Signature, error) {
	if s == nil {
		return airgap.Signature{}, errors.New("Agent CA signer is unavailable")
	}
	return airgap.Sign(s.privateKey, canonical)
}

// VerifyClientCertificate validates a Local Agent certificate against the
// configured Agent CA and its tenant-scoped SPIFFE identity at the build time.
func (s *Signer) VerifyClientCertificate(certificatePEM string, tenantID, agentID int64, at time.Time) (*x509.Certificate, error) {
	if s == nil || tenantID <= 0 || agentID <= 0 || at.IsZero() {
		return nil, errors.New("Agent certificate verification context is invalid")
	}
	block, trailing := pem.Decode([]byte(certificatePEM))
	if block == nil || block.Type != "CERTIFICATE" || len(trailing) != 0 {
		return nil, errors.New("Agent certificate PEM is invalid")
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, errors.New("Agent certificate DER is invalid")
	}
	roots := x509.NewCertPool()
	roots.AddCert(s.certificate)
	if _, err := certificate.Verify(x509.VerifyOptions{Roots: roots, CurrentTime: at, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}); err != nil {
		return nil, fmt.Errorf("verify Agent certificate chain: %w", err)
	}
	publicKey, ok := certificate.PublicKey.(*ecdsa.PublicKey)
	if !ok || publicKey.Curve == nil || publicKey.Curve.Params().Name != "P-256" {
		return nil, errors.New("Agent certificate must use ECDSA P-256")
	}
	expected := fmt.Sprintf("spiffe://appforge/tenant/%d/local-agent/%d", tenantID, agentID)
	if len(certificate.URIs) != 1 || certificate.URIs[0].String() != expected {
		return nil, errors.New("Agent certificate identity does not match package")
	}
	return certificate, nil
}

func randomSerial() *big.Int {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return big.NewInt(time.Now().UnixNano())
	}
	return serial
}
