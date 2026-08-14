package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"appforge/common/offlinelicense"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "generate-key":
		err = generateKey(os.Args[2:])
	case "issue":
		err = issue(os.Args[2:])
	case "verify":
		err = verify(os.Args[2:])
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: licensectl generate-key|issue|verify")
	os.Exit(2)
}

func generateKey(args []string) error {
	flags := flag.NewFlagSet("generate-key", flag.ContinueOnError)
	privatePath := flags.String("private", "", "private key PEM output")
	publicPath := flags.String("public", "", "public key PEM output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *privatePath == "" || *publicPath == "" {
		return errors.New("private and public output paths are required")
	}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return err
	}
	publicDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return err
	}
	if err := writeExclusive(*privatePath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER}), 0o600); err != nil {
		return err
	}
	return writeExclusive(*publicPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}), 0o644)
}

func issue(args []string) error {
	flags := flag.NewFlagSet("issue", flag.ContinueOnError)
	privatePath := flags.String("private", "", "vendor private key PEM")
	output := flags.String("output", "", "signed license JSON output")
	licenseID := flags.String("license-id", "", "stable license ID")
	customer := flags.String("customer", "", "customer name")
	deploymentID := flags.String("deployment-id", "", "customer deployment binding")
	modes := flags.String("modes", "private", "comma-separated dedicated,private,hybrid modes")
	features := flags.String("features", "local-agent,offline", "comma-separated features")
	notBefore := flags.String("not-before", "", "RFC3339 start time; defaults to now")
	notAfter := flags.String("not-after", "", "RFC3339 expiry time")
	validFor := flags.Duration("valid-for", 0, "validity duration used when not-after is omitted")
	sequence := flags.Uint64("sequence", 1, "monotonic license revision")
	maxTenants := flags.Int64("max-tenants", 1, "maximum tenants; -1 means unlimited")
	maxBuilders := flags.Int64("max-builders", 1, "maximum builders; -1 means unlimited")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *privatePath == "" || *output == "" || *licenseID == "" || *customer == "" || *deploymentID == "" || (*notAfter == "" && *validFor <= 0) {
		return errors.New("private, output, license-id, customer, deployment-id and either not-after or valid-for are required")
	}
	now := time.Now().UTC()
	start := now
	var err error
	if *notBefore != "" {
		start, err = time.Parse(time.RFC3339, *notBefore)
		if err != nil {
			return fmt.Errorf("parse not-before: %w", err)
		}
	}
	end := start.Add(*validFor)
	if *notAfter != "" {
		end, err = time.Parse(time.RFC3339, *notAfter)
		if err != nil {
			return fmt.Errorf("parse not-after: %w", err)
		}
	}
	privateKey, err := loadPrivateKey(*privatePath)
	if err != nil {
		return err
	}
	payload := offlinelicense.Payload{LicenseID: *licenseID, Customer: *customer, DeploymentID: *deploymentID,
		DeploymentModes: splitList(*modes), Features: splitList(*features), NotBefore: start.UnixMilli(),
		NotAfter: end.UnixMilli(), IssuedAt: now.UnixMilli(), Sequence: *sequence,
		MaxTenants: *maxTenants, MaxBuilders: *maxBuilders}
	envelope, err := offlinelicense.Sign(payload, privateKey)
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return err
	}
	return writeExclusive(*output, append(raw, '\n'), 0o600)
}

func verify(args []string) error {
	flags := flag.NewFlagSet("verify", flag.ContinueOnError)
	licensePath := flags.String("license", "", "signed license JSON")
	publicPath := flags.String("public", "", "vendor public key PEM")
	statePath := flags.String("state", "", "persistent verifier state")
	deploymentID := flags.String("deployment-id", "", "expected deployment binding")
	mode := flags.String("mode", "", "expected deployment mode")
	if err := flags.Parse(args); err != nil {
		return err
	}
	verified, err := offlinelicense.VerifyFile(offlinelicense.Config{LicenseFile: *licensePath,
		PublicKeyFile: *publicPath, StateFile: *statePath, DeploymentID: *deploymentID, DeploymentMode: *mode}, time.Now())
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(verified)
}

func loadPrivateKey(path string) (ed25519.PrivateKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, errors.New("private key PEM is invalid")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not Ed25519")
	}
	return key, nil
}

func writeExclusive(path string, data []byte, mode os.FileMode) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func splitList(value string) []string {
	result := make([]string, 0)
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			result = append(result, item)
		}
	}
	return result
}
