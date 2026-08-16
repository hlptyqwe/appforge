package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"appforge/common/remotesigner"
)

func main() {
	secretPath := flag.String("secret", "", "remote signer Secret JSON file")
	inputPath := flag.String("input", "", "aligned unsigned APK")
	outputPath := flag.String("output", "", "signed APK output")
	taskID := flag.Int64("task", 0, "build task ID")
	attempt := flag.Int("attempt", 0, "builder attempt")
	expectedCertificate := flag.String("certificate-sha256", "", "expected APK signer certificate SHA-256")
	nonce := flag.String("nonce", "", "optional fixed replay-test nonce")
	infoOnly := flag.Bool("info-only", false, "only validate signer info")
	timeout := flag.Duration("timeout", 2*time.Minute, "whole request timeout")
	flag.Parse()

	if strings.TrimSpace(*secretPath) == "" {
		fatal("secret file is required")
	}
	raw, err := os.ReadFile(*secretPath)
	if err != nil || len(raw) == 0 || len(raw) > 64<<10 {
		fatal("read bounded remote signer Secret failed")
	}
	secret, err := remotesigner.ParseSecret(raw)
	for index := range raw {
		raw[index] = 0
	}
	if err != nil {
		fatal(err.Error())
	}
	defer secret.Erase()
	client, err := remotesigner.NewClient(secret, 128<<20)
	if err != nil {
		fatal(err.Error())
	}
	if *timeout <= 0 || *timeout > 5*time.Minute {
		fatal("timeout must be between zero and five minutes")
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	info, err := client.Info(ctx)
	if err != nil {
		fatal(err.Error())
	}
	if *infoOnly {
		fmt.Printf("keyId=%s certificateSha256=%s\n", info.KeyID, info.CertificateSHA256)
		return
	}
	if *inputPath == "" || *outputPath == "" || *taskID <= 0 || *attempt <= 0 || len(*expectedCertificate) != 64 {
		fatal("signing arguments are incomplete")
	}
	result, err := client.SignFile(ctx, remotesigner.SignRequest{
		TaskID: *taskID, BuilderAttempt: int32(*attempt), UnsignedAPKPath: *inputPath,
		SignedAPKPath: *outputPath, CertificateSHA256: *expectedCertificate, Nonce: *nonce,
	})
	if err != nil {
		fatal(err.Error())
	}
	fmt.Printf("signedSize=%d signedSha256=%s nonce=%s\n", result.SizeBytes, result.SHA256, result.Nonce)
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, "remote signer acceptance client:", message)
	os.Exit(1)
}
