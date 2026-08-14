package agentpki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"strings"
	"testing"
	"time"
)

func TestSignCSRProducesScopedClientCertificate(t *testing.T) {
	signer, err := New("", "", 2*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{Subject: pkix.Name{CommonName: "ignored-client-name"}}, key)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := signer.SignCSR(string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})), 17, 23)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode([]byte(issued.PEM))
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if got := certificate.URIs[0].String(); got != "spiffe://appforge/tenant/17/local-agent/23" {
		t.Fatalf("identity URI = %q", got)
	}
	if certificate.Subject.CommonName != "local-agent-23" {
		t.Fatalf("common name = %q", certificate.Subject.CommonName)
	}
	if len(certificate.ExtKeyUsage) != 1 || certificate.ExtKeyUsage[0] != x509.ExtKeyUsageClientAuth {
		t.Fatalf("unexpected extended key usage: %v", certificate.ExtKeyUsage)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM([]byte(signer.CAPEM())) {
		t.Fatal("append CA")
	}
	if _, err := certificate.Verify(x509.VerifyOptions{Roots: roots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}); err != nil {
		t.Fatalf("verify client certificate: %v", err)
	}
	if len(issued.Fingerprint) != 64 || strings.ToLower(issued.Fingerprint) != issued.Fingerprint {
		t.Fatalf("invalid fingerprint %q", issued.Fingerprint)
	}
	if issued.NotAfter.Sub(time.Now()) > 2*time.Hour+time.Minute {
		t.Fatalf("certificate exceeded configured lifetime")
	}
}

func TestSignCSRRejectsInvalidIdentityAndKeyType(t *testing.T) {
	signer, err := New("", "", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := signer.SignCSR("invalid", 1, 1); err == nil {
		t.Fatal("expected malformed CSR rejection")
	}
	if _, err := signer.SignCSR("invalid", 0, 1); err == nil {
		t.Fatal("expected tenant identity rejection")
	}
}
