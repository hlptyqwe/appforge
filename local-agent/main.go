package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const version = "1.1.0"

const protocolVersion int32 = 3

type state struct {
	AgentID            int64  `json:"agentId"`
	GatewayURL         string `json:"gatewayUrl"`
	Certificate        string `json:"certificate"`
	PrivateKey         string `json:"privateKey"`
	ClientCA           string `json:"clientCa"`
	GatewayCA          string `json:"gatewayCa,omitempty"`
	CustomerStorageRef string `json:"customerStorageRef,omitempty"`
	Protocol           int32  `json:"protocol"`
	AgentVersion       string `json:"agentVersion"`
	LastTimestamp      int64  `json:"lastTimestamp"`
}

var (
	authStateMu          sync.Mutex
	authenticatedRequest sync.Mutex
)

type task struct {
	ID             int64  `json:"id"`
	TenantID       int64  `json:"tenant_id"`
	AppID          int64  `json:"app_id"`
	VersionID      int64  `json:"version_id"`
	BuilderAttempt int32  `json:"builder_attempt"`
	ChannelCode    string `json:"channel_code"`
	VersionCode    int64  `json:"version_code"`
	VersionName    string `json:"version_name"`
}

type claimResponse struct {
	Task               *task          `json:"task"`
	ArtifactMode       int32          `json:"artifact_mode"`
	CustomerStorageRef string         `json:"customer_storage_ref,omitempty"`
	Bundle             *buildManifest `json:"bundle"`
}

type buildManifest struct {
	SchemaVersion           int32         `json:"schema_version"`
	Task                    *task         `json:"task"`
	PackageName             string        `json:"package_name"`
	APIHost                 string        `json:"api_host"`
	ChannelName             string        `json:"channel_name"`
	LandingURL              string        `json:"landing_url"`
	KeyAlias                string        `json:"key_alias"`
	SigningSecretRef        string        `json:"signing_secret_ref,omitempty"`
	SignerCertificateSHA256 string        `json:"signer_certificate_sha256"`
	BrandingSnapshotJSON    string        `json:"branding_snapshot_json"`
	TemplateSnapshotJSON    string        `json:"template_snapshot_json"`
	Inputs                  []buildInput  `json:"inputs"`
	Outputs                 []buildOutput `json:"outputs,omitempty"`
	BlockedReason           string        `json:"blocked_reason"`
}

type buildInput struct {
	Role              string `json:"role"`
	ObjectID          int64  `json:"object_id"`
	ObjectType        int32  `json:"object_type"`
	OriginalName      string `json:"original_name"`
	ContentType       string `json:"content_type"`
	SizeBytes         int64  `json:"size_bytes"`
	SHA256            string `json:"sha256"`
	StorageMode       int32  `json:"storage_mode,omitempty"`
	OwnerAgentID      int64  `json:"owner_agent_id,omitempty"`
	LocalPath         string `json:"local_path,omitempty"`
	DownloadPath      string `json:"download_path,omitempty"`
	CustomerReference string `json:"customer_reference,omitempty"`
}

type buildOutput struct {
	Role       string `json:"role"`
	UploadPath string `json:"upload_path"`
	ExpiresAt  int64  `json:"expires_at"`
}

type buildResult struct {
	APKPath      string `json:"apkPath"`
	APKReference string `json:"apkReference"`
	APKSHA256    string `json:"apkSha256"`
	APKSize      int64  `json:"apkSize"`
	LogPath      string `json:"logPath"`
	LogReference string `json:"logReference"`
	LogSHA256    string `json:"logSha256"`
	LogSize      int64  `json:"logSize"`
	Error        string `json:"error"`
}

type localSigningSecret struct {
	KeystorePassword string `json:"keystorePassword"`
	KeyPassword      string `json:"keyPassword"`
}

type certificateResponse struct {
	Data struct {
		ID                 int64  `json:"id"`
		CustomerStorageRef string `json:"customer_storage_ref"`
	} `json:"data"`
	Certificate struct {
		SerialNumber   string `json:"serial_number"`
		CertificatePEM string `json:"certificate_pem"`
		NotAfter       int64  `json:"not_after"`
	} `json:"certificate"`
	CAPEM string `json:"ca_certificate_pem"`
}

type artifactRefreshResponse struct {
	Bundle *buildManifest `json:"bundle"`
}

type artifactUploadResponse struct {
	Reference string `json:"reference"`
	SHA256    string `json:"sha256"`
	SizeBytes int64  `json:"size_bytes"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "register":
		if err := registerCommand(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
	case "run":
		if err := runCommand(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
	case "version":
		fmt.Printf("appforge-local-agent %s protocol=%d\n", version, protocolVersion)
	case "health":
		if err := healthCommand(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
	case "secret-import":
		if err := secretImportCommand(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
	case "customer-storage-secret-import":
		if err := customerStorageSecretImportCommand(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
	case "customer-storage-import":
		if err := customerStorageImportCommand(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
	case "air-gapped-build":
		if err := airGappedBuildCommand(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
	case "offline-sign":
		if err := offlineSignCommand(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
	case "offline-verify":
		if err := offlineVerifyCommand(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: appforge-local-agent register|run|health|version|secret-import|customer-storage-secret-import|customer-storage-import|air-gapped-build|offline-sign|offline-verify")
	os.Exit(2)
}

func registerCommand(args []string) error {
	flags := flag.NewFlagSet("register", flag.ContinueOnError)
	controlURL := flags.String("control-url", "", "control-plane registration URL")
	controlCA := flags.String("control-ca", "", "control-plane server CA PEM")
	gatewayURL := flags.String("gateway-url", "", "mTLS Agent gateway URL")
	gatewayCA := flags.String("gateway-ca", "", "gateway server CA PEM; defaults to the Agent CA returned by registration")
	token := flags.String("token", "", "one-time registration token")
	tokenFile := flags.String("token-file", "", "private file containing the one-time registration token")
	tokenStdin := flags.Bool("token-stdin", false, "read the one-time registration token from standard input")
	stateDir := flags.String("state-dir", defaultStateDir(), "Agent local state directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	tokenSources := 0
	if *token != "" {
		tokenSources++
	}
	if *tokenFile != "" {
		tokenSources++
	}
	if *tokenStdin {
		tokenSources++
	}
	if *controlURL == "" || *gatewayURL == "" || tokenSources != 1 {
		return errors.New("control-url, gateway-url and exactly one registration token source are required")
	}
	registrationToken := strings.TrimSpace(*token)
	if *tokenFile != "" {
		value, err := readPrivateToken(*tokenFile)
		if err != nil {
			return err
		}
		registrationToken = value
	}
	if *tokenStdin {
		raw, err := io.ReadAll(io.LimitReader(os.Stdin, 4097))
		if err != nil {
			return err
		}
		registrationToken = strings.TrimSpace(string(raw))
	}
	if registrationToken == "" || len(registrationToken) > 4096 {
		return errors.New("registration token is empty or too large")
	}
	absoluteStateDir, err := filepath.Abs(*stateDir)
	if err != nil {
		return err
	}
	*stateDir = absoluteStateDir
	if err := os.MkdirAll(*stateDir, 0o700); err != nil {
		return err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{Subject: pkix.Name{CommonName: "appforge-local-agent"}}, key)
	if err != nil {
		return err
	}
	csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})
	payload := map[string]any{"registration_token": registrationToken, "csr_pem": string(csrPEM), "agent_version": version, "protocol_version": protocolVersion, "nonce": newNonce(), "timestamp": time.Now().UnixMilli()}
	var response certificateResponse
	registrationClient, err := clientWithServerCA(*controlCA)
	if err != nil {
		return err
	}
	if err := postJSON(context.Background(), registrationClient, strings.TrimRight(*controlURL, "/")+"/public/v1/local-agent/register", payload, &response); err != nil {
		return err
	}
	if response.Data.ID <= 0 || response.Certificate.CertificatePEM == "" || response.CAPEM == "" {
		return errors.New("registration response is incomplete")
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	keyPath := filepath.Join(*stateDir, "client.key")
	certPath := filepath.Join(*stateDir, "client.crt")
	caPath := filepath.Join(*stateDir, "agent-ca.crt")
	gatewayCAPath := filepath.Join(*stateDir, "gateway-ca.crt")
	if err := writePrivateFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})); err != nil {
		return err
	}
	if err := writePrivateFile(certPath, []byte(response.Certificate.CertificatePEM)); err != nil {
		return err
	}
	if err := writePrivateFile(caPath, []byte(response.CAPEM)); err != nil {
		return err
	}
	gatewayCAPEM := []byte(response.CAPEM)
	if strings.TrimSpace(*gatewayCA) != "" {
		gatewayCAPEM, err = os.ReadFile(*gatewayCA)
		if err != nil {
			return fmt.Errorf("read gateway CA: %w", err)
		}
	}
	if err := writePrivateFile(gatewayCAPath, gatewayCAPEM); err != nil {
		return err
	}
	current := state{AgentID: response.Data.ID, GatewayURL: strings.TrimRight(*gatewayURL, "/"), Certificate: certPath,
		PrivateKey: keyPath, ClientCA: caPath, GatewayCA: gatewayCAPath, CustomerStorageRef: response.Data.CustomerStorageRef,
		Protocol: protocolVersion, AgentVersion: version}
	encoded, _ := json.MarshalIndent(current, "", "  ")
	if err := writePrivateFile(filepath.Join(*stateDir, "state.json"), encoded); err != nil {
		return err
	}
	fmt.Printf("Local Agent %d registered; state saved in %s\n", response.Data.ID, *stateDir)
	return nil
}

func healthCommand(args []string) error {
	flags := flag.NewFlagSet("health", flag.ContinueOnError)
	stateDir := flags.String("state-dir", defaultStateDir(), "Agent local state directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	stateRoot, err := filepath.Abs(*stateDir)
	if err != nil {
		return err
	}
	current, err := loadState(stateRoot)
	if err != nil {
		return fmt.Errorf("load Agent state: %w", err)
	}
	if err := validateLocalState(stateRoot, &current); err != nil {
		return err
	}
	client, err := mtlsClient(&current)
	if err != nil {
		return fmt.Errorf("validate Agent mTLS state: %w", err)
	}
	if transport, ok := client.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
	fmt.Printf("healthy agent=%d version=%s protocol=%d\n", current.AgentID, version, protocolVersion)
	return nil
}

func secretImportCommand(args []string) error {
	flags := flag.NewFlagSet("secret-import", flag.ContinueOnError)
	secretRoot := flags.String("secret-root", "/etc/appforge/local-secrets", "absolute Local Agent Secret root")
	name := flags.String("name", "", "Secret JSON filename")
	inputStdin := flags.Bool("input-stdin", false, "read strict Secret JSON from standard input")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if !*inputStdin {
		return errors.New("input-stdin is required")
	}
	if !filepath.IsAbs(*secretRoot) {
		return errors.New("Secret root must be absolute")
	}
	cleanName := filepath.Base(strings.TrimSpace(*name))
	if cleanName == "" || cleanName == "." || cleanName != strings.TrimSpace(*name) ||
		!strings.HasSuffix(strings.ToLower(cleanName), ".json") {
		return errors.New("Secret name must be a single .json filename")
	}
	if err := os.MkdirAll(*secretRoot, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(*secretRoot, 0o700); err != nil {
		return err
	}
	secret, err := decodeLocalSigningSecret(os.Stdin)
	if err != nil {
		return err
	}
	defer secret.erase()
	encoded, err := json.Marshal(secret)
	if err != nil {
		return err
	}
	if err := writePrivateFile(filepath.Join(*secretRoot, cleanName), encoded); err != nil {
		return err
	}
	fmt.Printf("Local signing Secret imported: local-file:///%s\n", cleanName)
	return nil
}

func runCommand(args []string) error {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	stateDir := flags.String("state-dir", defaultStateDir(), "Agent local state directory")
	executor := flags.String("executor", "appforge-local-build", "fixed local build executable")
	secretRoot := flags.String("secret-root", "/etc/appforge/local-secrets", "absolute root for local-file signing Secret references")
	poll := flags.Duration("poll", 2*time.Second, "claim polling interval")
	lease := flags.Int("lease-seconds", 120, "task lease seconds")
	maxConcurrent := flags.Int("max-concurrency", 1, "maximum local builds")
	rotateBefore := flags.Duration("rotate-before", 6*time.Hour, "rotate the mTLS certificate this long before expiry")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *maxConcurrent < 1 || *maxConcurrent > 64 {
		return errors.New("max-concurrency must be between 1 and 64")
	}
	if *rotateBefore <= 0 {
		return errors.New("rotate-before must be greater than zero")
	}
	absoluteStateDir, err := filepath.Abs(*stateDir)
	if err != nil {
		return err
	}
	*stateDir = absoluteStateDir
	current, err := loadState(*stateDir)
	if err != nil {
		return err
	}
	if err := validateLocalState(*stateDir, &current); err != nil {
		return err
	}
	if current.Protocol != protocolVersion || current.AgentVersion != version {
		current.Protocol = protocolVersion
		current.AgentVersion = version
		encoded, _ := json.MarshalIndent(current, "", "  ")
		if err := writePrivateFile(filepath.Join(*stateDir, "state.json"), encoded); err != nil {
			return err
		}
	}
	client, err := mtlsClient(&current)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	var active sync.WaitGroup
	slots := make(chan struct{}, *maxConcurrent)
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	lastHeartbeat := time.Time{}
	nextRotationCheck := time.Time{}
	for {
		select {
		case <-ctx.Done():
			active.Wait()
			return nil
		case <-heartbeat.C:
			if err := sendHeartbeat(ctx, client, &current, *stateDir, *maxConcurrent); err != nil {
				log.Printf("heartbeat failed: %v", err)
			}
			lastHeartbeat = time.Now()
		default:
		}
		if len(slots) == 0 && time.Now().After(nextRotationCheck) {
			rotatedClient, rotated, rotateErr := rotateCertificateIfDue(ctx, client, &current, *stateDir, *rotateBefore)
			if rotateErr != nil {
				log.Printf("certificate rotation check failed: %v", rotateErr)
			} else if rotated {
				client = rotatedClient
				log.Printf("mTLS certificate rotated successfully")
			}
			nextRotationCheck = time.Now().Add(time.Minute)
		}
		if lastHeartbeat.IsZero() {
			_ = sendHeartbeat(ctx, client, &current, *stateDir, *maxConcurrent)
			lastHeartbeat = time.Now()
		}
		select {
		case slots <- struct{}{}:
		default:
			if !wait(ctx, *poll) {
				continue
			}
			continue
		}
		var claim claimResponse
		err := postAuthenticatedJSON(ctx, client, &current, *stateDir, current.GatewayURL+"/v1/claim", map[string]any{"lease_seconds": *lease}, &claim)
		if err != nil {
			<-slots
			log.Printf("claim failed: %v", err)
			if !wait(ctx, *poll) {
				continue
			}
			continue
		}
		if claim.Task == nil || claim.Task.ID <= 0 {
			<-slots
			if !wait(ctx, *poll) {
				continue
			}
			continue
		}
		active.Add(1)
		go func(item *task, mode int32, customerStorageRef string, bundle *buildManifest) {
			defer active.Done()
			defer func() { <-slots }()
			if err := executeTask(ctx, client, &current, *stateDir, *executor, *secretRoot, *lease, item, mode, customerStorageRef, bundle); err != nil {
				log.Printf("task %d failed: %v", item.ID, err)
			}
		}(claim.Task, claim.ArtifactMode, claim.CustomerStorageRef, claim.Bundle)
	}
}

func executeTask(parent context.Context, client *http.Client, current *state, stateDir, executor, secretRoot string, lease int, item *task, mode int32, customerStorageRef string, bundle *buildManifest) error {
	if bundle == nil || bundle.SchemaVersion != protocolVersion {
		message := "LOCAL_TASK_BUNDLE_REQUIRED"
		_ = postAuthenticatedJSON(parent, client, current, stateDir, current.GatewayURL+"/v1/tasks/fail", map[string]any{"task_id": item.ID, "builder_attempt": item.BuilderAttempt, "error_message": message}, nil)
		return errors.New(message)
	}
	if bundle.BlockedReason != "" {
		_ = postAuthenticatedJSON(parent, client, current, stateDir, current.GatewayURL+"/v1/tasks/fail", map[string]any{"task_id": item.ID, "builder_attempt": item.BuilderAttempt, "error_message": bundle.BlockedReason}, nil)
		return errors.New(bundle.BlockedReason)
	}
	secret, err := resolveLocalSigningSecret(secretRoot, bundle.SigningSecretRef)
	if err != nil {
		message := "LOCAL_SIGNING_SECRET_RESOLUTION_FAILED"
		_ = postAuthenticatedJSON(parent, client, current, stateDir, current.GatewayURL+"/v1/tasks/fail", map[string]any{"task_id": item.ID, "builder_attempt": item.BuilderAttempt, "error_message": message}, nil)
		return fmt.Errorf("%s: %w", message, err)
	}
	defer secret.erase()
	workDir, err := os.MkdirTemp("", "appforge-local-task-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir)
	ctx, cancel := context.WithCancel(parent)
	renewDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(time.Duration(lease/3) * time.Second)
		defer ticker.Stop()
		defer close(renewDone)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := postAuthenticatedJSON(ctx, client, current, stateDir, current.GatewayURL+"/v1/tasks/renew", map[string]any{"task_id": item.ID, "builder_attempt": item.BuilderAttempt, "lease_seconds": lease}, nil); err != nil {
					log.Printf("renew task %d failed: %v", item.ID, err)
					cancel()
					return
				}
			}
		}
	}()
	defer func() {
		cancel()
		<-renewDone
	}()
	var customerSecret *customerStorageSecret
	var customerStore customerObjectStore
	if mode == 1 {
		if err := downloadArtifactInputs(ctx, client, current.GatewayURL, workDir, bundle); err != nil {
			refreshed, refreshErr := refreshArtifactBundle(ctx, client, current, stateDir, item)
			if refreshErr != nil {
				message := "LOCAL_ARTIFACT_INPUT_DOWNLOAD_FAILED"
				_ = postTaskFailure(parent, client, current, stateDir, item, message, nil)
				return fmt.Errorf("%s: %w", message, err)
			}
			bundle.Outputs = refreshed.Outputs
			bundle.Inputs = refreshed.Inputs
			if err := downloadArtifactInputs(ctx, client, current.GatewayURL, workDir, bundle); err != nil {
				message := "LOCAL_ARTIFACT_INPUT_INTEGRITY_FAILED"
				_ = postTaskFailure(parent, client, current, stateDir, item, message, nil)
				return fmt.Errorf("%s: %w", message, err)
			}
		}
	} else if mode == 2 {
		if strings.TrimSpace(customerStorageRef) == "" || customerStorageRef != current.CustomerStorageRef {
			message := "LOCAL_CUSTOMER_STORAGE_REFERENCE_MISMATCH"
			_ = postTaskFailure(parent, client, current, stateDir, item, message, nil)
			return errors.New(message)
		}
		customerSecret, customerStore, err = resolveCustomerStorage(secretRoot, customerStorageRef)
		if err != nil {
			message := "LOCAL_CUSTOMER_STORAGE_SECRET_RESOLUTION_FAILED"
			_ = postTaskFailure(parent, client, current, stateDir, item, message, nil)
			return fmt.Errorf("%s: %w", message, err)
		}
		defer customerSecret.erase()
		if err := downloadCustomerArtifactInputs(ctx, customerStore, current.AgentID, customerSecret.Prefix, workDir, bundle); err != nil {
			message := "LOCAL_CUSTOMER_STORAGE_INPUT_INTEGRITY_FAILED"
			_ = postTaskFailure(parent, client, current, stateDir, item, message, nil)
			return fmt.Errorf("%s: %w", message, err)
		}
	} else {
		message := "LOCAL_ARTIFACT_MODE_UNSUPPORTED"
		_ = postTaskFailure(parent, client, current, stateDir, item, message, nil)
		return errors.New(message)
	}
	taskFile := filepath.Join(workDir, "task.json")
	resultFile := filepath.Join(workDir, "result.json")
	executorBundle := *bundle
	executorBundle.SigningSecretRef = ""
	executorBundle.Outputs = nil
	executorBundle.Inputs = append([]buildInput(nil), bundle.Inputs...)
	for index := range executorBundle.Inputs {
		executorBundle.Inputs[index].DownloadPath = ""
		executorBundle.Inputs[index].CustomerReference = ""
		executorBundle.Inputs[index].StorageMode = 0
		executorBundle.Inputs[index].OwnerAgentID = 0
	}
	encoded, _ := json.MarshalIndent(map[string]any{"task": item, "artifactMode": mode, "bundle": &executorBundle}, "", "  ")
	if err := os.WriteFile(taskFile, encoded, 0o600); err != nil {
		return err
	}
	_ = postAuthenticatedJSON(ctx, client, current, stateDir, current.GatewayURL+"/v1/tasks/progress", map[string]any{"task_id": item.ID, "builder_attempt": item.BuilderAttempt, "status": 2, "progress": 5, "message": "local executor started"}, nil)
	command := exec.CommandContext(ctx, executor, "--task", taskFile, "--result", resultFile)
	command.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + workDir, "TMPDIR=" + workDir,
		"APPFORGE_TASK_ID=" + fmt.Sprint(item.ID), "APPFORGE_ARTIFACT_MODE=" + fmt.Sprint(mode),
		"APPFORGE_KEYSTORE_PASSWORD=" + secret.KeystorePassword, "APPFORGE_KEY_PASSWORD=" + secret.KeyPassword}
	output, runErr := command.CombinedOutput()
	var result buildResult
	if data, err := os.ReadFile(resultFile); err == nil {
		_ = json.Unmarshal(data, &result)
	}
	if runErr != nil || result.Error != "" {
		if mode == 1 && strings.TrimSpace(result.LogPath) != "" {
			if refreshed, refreshErr := refreshArtifactBundle(parent, client, current, stateDir, item); refreshErr == nil {
				bundle.Outputs = refreshed.Outputs
				if uploaded, uploadErr := uploadArtifactOutputWithRefresh(parent, client, current, stateDir, item, workDir, bundle, "build_log", result.LogPath); uploadErr == nil {
					result.LogReference, result.LogSHA256, result.LogSize = uploaded.Reference, uploaded.SHA256, uploaded.SizeBytes
				}
			}
		} else if mode == 2 && strings.TrimSpace(result.LogPath) != "" {
			if uploaded, uploadErr := uploadCustomerArtifactOutput(parent, customerStore, current.AgentID, customerSecret.Prefix,
				item, workDir, "build_log", result.LogPath); uploadErr == nil {
				result.LogReference, result.LogSHA256, result.LogSize = uploaded.Reference, uploaded.SHA256, uploaded.SizeBytes
			}
		}
		message := result.Error
		if message == "" {
			message = runErr.Error()
		}
		if len(output) > 0 {
			message += "; executor output sha256=" + digestBytes(output)
		}
		failErr := postTaskFailure(parent, client, current, stateDir, item, message, &result)
		if failErr != nil {
			return failErr
		}
		return errors.New(message)
	}
	if mode == 1 {
		refreshed, err := refreshArtifactBundle(parent, client, current, stateDir, item)
		if err != nil {
			message := "LOCAL_ARTIFACT_TICKET_REFRESH_FAILED"
			_ = postTaskFailure(parent, client, current, stateDir, item, message, nil)
			return fmt.Errorf("%s: %w", message, err)
		}
		bundle.Outputs = refreshed.Outputs
		if strings.TrimSpace(result.APKPath) == "" {
			message := "LOCAL_ARTIFACT_OUTPUT_PATH_REQUIRED"
			_ = postTaskFailure(parent, client, current, stateDir, item, message, nil)
			return errors.New(message)
		}
		uploadedAPK, err := uploadArtifactOutputWithRefresh(parent, client, current, stateDir, item, workDir, bundle, "built_apk", result.APKPath)
		if err != nil {
			message := "LOCAL_ARTIFACT_APK_UPLOAD_FAILED"
			_ = postTaskFailure(parent, client, current, stateDir, item, message, nil)
			return fmt.Errorf("%s: %w", message, err)
		}
		result.APKReference, result.APKSHA256, result.APKSize = uploadedAPK.Reference, uploadedAPK.SHA256, uploadedAPK.SizeBytes
		if strings.TrimSpace(result.LogPath) != "" {
			uploadedLog, err := uploadArtifactOutputWithRefresh(parent, client, current, stateDir, item, workDir, bundle, "build_log", result.LogPath)
			if err != nil {
				message := "LOCAL_ARTIFACT_LOG_UPLOAD_FAILED"
				_ = postTaskFailure(parent, client, current, stateDir, item, message, nil)
				return fmt.Errorf("%s: %w", message, err)
			}
			result.LogReference, result.LogSHA256, result.LogSize = uploadedLog.Reference, uploadedLog.SHA256, uploadedLog.SizeBytes
		}
	} else if mode == 2 {
		if strings.TrimSpace(result.APKPath) == "" {
			message := "LOCAL_CUSTOMER_STORAGE_OUTPUT_PATH_REQUIRED"
			_ = postTaskFailure(parent, client, current, stateDir, item, message, nil)
			return errors.New(message)
		}
		uploadedAPK, err := uploadCustomerArtifactOutput(parent, customerStore, current.AgentID, customerSecret.Prefix,
			item, workDir, "built_apk", result.APKPath)
		if err != nil {
			message := "LOCAL_CUSTOMER_STORAGE_APK_UPLOAD_FAILED"
			_ = postTaskFailure(parent, client, current, stateDir, item, message, nil)
			return fmt.Errorf("%s: %w", message, err)
		}
		result.APKReference, result.APKSHA256, result.APKSize = uploadedAPK.Reference, uploadedAPK.SHA256, uploadedAPK.SizeBytes
		if strings.TrimSpace(result.LogPath) != "" {
			uploadedLog, err := uploadCustomerArtifactOutput(parent, customerStore, current.AgentID, customerSecret.Prefix,
				item, workDir, "build_log", result.LogPath)
			if err != nil {
				message := "LOCAL_CUSTOMER_STORAGE_LOG_UPLOAD_FAILED"
				_ = postTaskFailure(parent, client, current, stateDir, item, message, nil)
				return fmt.Errorf("%s: %w", message, err)
			}
			result.LogReference, result.LogSHA256, result.LogSize = uploadedLog.Reference, uploadedLog.SHA256, uploadedLog.SizeBytes
		}
	}
	if strings.TrimSpace(result.APKReference) == "" {
		message := "LOCAL_ARTIFACT_UPLOAD_REQUIRED"
		failErr := postAuthenticatedJSON(parent, client, current, stateDir, current.GatewayURL+"/v1/tasks/fail", map[string]any{"task_id": item.ID, "builder_attempt": item.BuilderAttempt, "error_message": message}, nil)
		if failErr != nil {
			return failErr
		}
		return errors.New(message)
	}
	return postAuthenticatedJSON(parent, client, current, stateDir, current.GatewayURL+"/v1/tasks/complete", map[string]any{"task_id": item.ID, "builder_attempt": item.BuilderAttempt, "apk_reference": result.APKReference, "apk_sha256": result.APKSHA256, "apk_size": result.APKSize, "log_reference": result.LogReference, "log_sha256": result.LogSHA256, "log_size": result.LogSize}, nil)
}

func postTaskFailure(ctx context.Context, client *http.Client, current *state, stateDir string, item *task, message string, result *buildResult) error {
	payload := map[string]any{"task_id": item.ID, "builder_attempt": item.BuilderAttempt, "error_message": truncateAgentMessage(message)}
	if result != nil {
		payload["log_reference"], payload["log_sha256"], payload["log_size"] = result.LogReference, result.LogSHA256, result.LogSize
	}
	return postAuthenticatedJSON(ctx, client, current, stateDir, current.GatewayURL+"/v1/tasks/fail", payload, nil)
}

func truncateAgentMessage(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 1900 {
		return value[:1900]
	}
	if value == "" {
		return "LOCAL_EXECUTOR_FAILED"
	}
	return value
}

func refreshArtifactBundle(ctx context.Context, client *http.Client, current *state, stateDir string, item *task) (*buildManifest, error) {
	var response artifactRefreshResponse
	err := postAuthenticatedJSON(ctx, client, current, stateDir, current.GatewayURL+"/v1/artifacts/refresh", map[string]any{
		"task_id": item.ID, "builder_attempt": item.BuilderAttempt,
	}, &response)
	if err != nil {
		return nil, err
	}
	if response.Bundle == nil || response.Bundle.SchemaVersion != protocolVersion {
		return nil, errors.New("Artifact ticket refresh response is incomplete")
	}
	return response.Bundle, nil
}

func downloadArtifactInputs(ctx context.Context, client *http.Client, gatewayURL, workDir string, bundle *buildManifest) error {
	if bundle == nil || len(bundle.Inputs) == 0 {
		return errors.New("Artifact input manifest is empty")
	}
	inputDir := filepath.Join(workDir, "inputs")
	if err := os.RemoveAll(inputDir); err != nil {
		return err
	}
	if err := os.MkdirAll(inputDir, 0o700); err != nil {
		return err
	}
	for index := range bundle.Inputs {
		input := &bundle.Inputs[index]
		transferURL, err := gatewayArtifactURL(gatewayURL, input.DownloadPath, "/v1/artifacts/download/")
		if err != nil {
			return fmt.Errorf("%s download ticket: %w", input.Role, err)
		}
		extension := strings.ToLower(filepath.Ext(strings.TrimSpace(input.OriginalName)))
		if len(extension) > 16 {
			extension = ""
		}
		role := safeFileSuffix(input.Role)
		if role == "" {
			role = "input"
		}
		target := filepath.Join(inputDir, fmt.Sprintf("%02d-%s-%d%s", index, role, input.ObjectID, extension))
		if err := downloadArtifactFile(ctx, client, transferURL, target, input.SizeBytes, input.SHA256); err != nil {
			return fmt.Errorf("%s input: %w", input.Role, err)
		}
		input.LocalPath = target
	}
	return nil
}

func downloadArtifactFile(ctx context.Context, client *http.Client, transferURL, target string, expectedSize int64, expectedDigest string) error {
	if expectedSize <= 0 || !validAgentSHA256(expectedDigest) {
		return errors.New("input integrity metadata is invalid")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, transferURL, nil)
	if err != nil {
		return err
	}
	resp, err := artifactHTTPClient(client).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		return fmt.Errorf("control plane returned %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	if resp.ContentLength >= 0 && resp.ContentLength != expectedSize {
		return errors.New("download response Content-Length mismatch")
	}
	if headerDigest := strings.ToLower(strings.TrimSpace(resp.Header.Get("X-AppForge-Sha256"))); headerDigest != "" && headerDigest != strings.ToLower(strings.TrimSpace(expectedDigest)) {
		return errors.New("download response SHA-256 header mismatch")
	}
	file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	hasher := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(file, hasher), io.LimitReader(resp.Body, expectedSize+1))
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil || written != expectedSize ||
		hex.EncodeToString(hasher.Sum(nil)) != strings.ToLower(strings.TrimSpace(expectedDigest)) {
		_ = os.Remove(target)
		return errors.New("downloaded input size or SHA-256 mismatch")
	}
	return nil
}

func uploadArtifactOutputWithRefresh(ctx context.Context, client *http.Client, current *state, stateDir string, item *task, workDir string, bundle *buildManifest, role, path string) (*artifactUploadResponse, error) {
	result, err := uploadArtifactOutput(ctx, client, current.GatewayURL, workDir, bundle, role, path)
	if err == nil {
		return result, nil
	}
	refreshed, refreshErr := refreshArtifactBundle(ctx, client, current, stateDir, item)
	if refreshErr != nil {
		return nil, fmt.Errorf("upload failed and ticket refresh failed: %w", err)
	}
	bundle.Outputs = refreshed.Outputs
	return uploadArtifactOutput(ctx, client, current.GatewayURL, workDir, bundle, role, path)
}

func uploadArtifactOutput(ctx context.Context, client *http.Client, gatewayURL, workDir string, bundle *buildManifest, role, path string) (*artifactUploadResponse, error) {
	var output *buildOutput
	for index := range bundle.Outputs {
		if bundle.Outputs[index].Role == role {
			output = &bundle.Outputs[index]
			break
		}
	}
	if output == nil {
		return nil, errors.New("Artifact upload ticket is missing")
	}
	transferURL, err := gatewayArtifactURL(gatewayURL, output.UploadPath, "/v1/artifacts/upload/")
	if err != nil {
		return nil, err
	}
	absolute, err := privateArtifactOutputPath(workDir, path)
	if err != nil {
		return nil, err
	}
	size, digest, err := localAgentFileDigest(absolute)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(absolute)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, transferURL, file)
	if err != nil {
		return nil, err
	}
	req.ContentLength = size
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-AppForge-Sha256", digest)
	resp, err := artifactHTTPClient(client).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("control plane returned %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	var result artifactUploadResponse
	if err := json.Unmarshal(data, &result); err != nil || result.Reference == "" || result.SizeBytes != size || result.SHA256 != digest {
		return nil, errors.New("control-plane Artifact upload response is invalid")
	}
	return &result, nil
}

func privateArtifactOutputPath(workDir, artifactPath string) (string, error) {
	absolute, err := filepath.Abs(artifactPath)
	if err != nil {
		return "", err
	}
	root, err := filepath.EvalSymlinks(workDir)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil || (resolved != root && !strings.HasPrefix(resolved, root+string(filepath.Separator))) {
		return "", errors.New("Artifact output path escapes task directory")
	}
	info, err := os.Lstat(absolute)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 || info.Size() <= 0 {
		return "", errors.New("Artifact output must be a private regular file")
	}
	return absolute, nil
}

func gatewayArtifactURL(gatewayURL, transferPath, requiredPrefix string) (string, error) {
	if !strings.HasPrefix(transferPath, requiredPrefix) || strings.Contains(strings.TrimPrefix(transferPath, requiredPrefix), "/") {
		return "", errors.New("transfer path is outside the fixed Gateway route")
	}
	base, err := url.Parse(gatewayURL)
	if err != nil || base.Scheme != "https" || base.Host == "" {
		return "", errors.New("Gateway URL is invalid")
	}
	relative, err := url.Parse(transferPath)
	if err != nil || relative.IsAbs() || relative.Host != "" || relative.RawQuery != "" || relative.Fragment != "" {
		return "", errors.New("transfer path is invalid")
	}
	return base.ResolveReference(relative).String(), nil
}

func localAgentFileDigest(path string) (int64, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, "", err
	}
	defer file.Close()
	hasher := sha256.New()
	size, err := io.Copy(hasher, file)
	if err != nil {
		return 0, "", err
	}
	return size, hex.EncodeToString(hasher.Sum(nil)), nil
}

func validAgentSHA256(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

func artifactHTTPClient(client *http.Client) *http.Client {
	copy := *client
	copy.Timeout = 30 * time.Minute
	return &copy
}

func sendHeartbeat(ctx context.Context, client *http.Client, current *state, stateDir string, maxConcurrency int) error {
	return postAuthenticatedJSON(ctx, client, current, stateDir, current.GatewayURL+"/v1/heartbeat", map[string]any{"agent_version": version, "protocol_version": protocolVersion, "capabilities": []map[string]string{{"capability_key": "apk", "capability_value": "true"}, {"capability_key": "max_concurrency", "capability_value": fmt.Sprint(maxConcurrency)}}, "running_task_ids": []int64{}}, nil)
}

func rotateCertificateIfDue(ctx context.Context, client *http.Client, current *state, stateDir string, rotateBefore time.Duration) (*http.Client, bool, error) {
	due, err := certificateExpiresWithin(current.Certificate, rotateBefore, time.Now())
	if err != nil || !due {
		return client, false, err
	}
	key, csrPEM, err := newAgentCSR()
	if err != nil {
		return client, false, err
	}
	var response certificateResponse
	payload := map[string]any{"csr_pem": string(csrPEM)}
	if err := postAuthenticatedJSON(ctx, client, current, stateDir, current.GatewayURL+"/v1/certificates/rotate", payload, &response); err != nil {
		return client, false, err
	}
	if response.Certificate.CertificatePEM == "" {
		return client, false, errors.New("certificate rotation response is incomplete")
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return client, false, err
	}
	suffix := safeFileSuffix(response.Certificate.SerialNumber)
	if suffix == "" {
		suffix = fmt.Sprint(time.Now().UnixMilli())
	}
	keyPath := filepath.Join(stateDir, "client-"+suffix+".key")
	certPath := filepath.Join(stateDir, "client-"+suffix+".crt")
	if err := writePrivateFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})); err != nil {
		return client, false, err
	}
	if err := writePrivateFile(certPath, []byte(response.Certificate.CertificatePEM)); err != nil {
		return client, false, err
	}

	authStateMu.Lock()
	current.PrivateKey = keyPath
	current.Certificate = certPath
	encoded, marshalErr := json.MarshalIndent(current, "", "  ")
	if marshalErr == nil {
		marshalErr = writePrivateFile(filepath.Join(stateDir, "state.json"), encoded)
	}
	authStateMu.Unlock()
	if marshalErr != nil {
		return client, false, marshalErr
	}
	rotatedClient, err := mtlsClient(current)
	if err != nil {
		return client, false, err
	}
	if transport, ok := client.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
	return rotatedClient, true, nil
}

func certificateExpiresWithin(path string, before time.Duration, now time.Time) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	block, _ := pem.Decode(raw)
	if block == nil || block.Type != "CERTIFICATE" {
		return false, errors.New("client certificate PEM is invalid")
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false, err
	}
	return !certificate.NotAfter.After(now.Add(before)), nil
}

func newAgentCSR() (*ecdsa.PrivateKey, []byte, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{Subject: pkix.Name{CommonName: "appforge-local-agent"}}, key)
	if err != nil {
		return nil, nil, err
	}
	return key, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER}), nil
}

func safeFileSuffix(value string) string {
	var result strings.Builder
	for _, current := range value {
		if current >= 'a' && current <= 'z' || current >= 'A' && current <= 'Z' || current >= '0' && current <= '9' || current == '-' || current == '_' {
			result.WriteRune(current)
		}
	}
	return result.String()
}

func resolveLocalSigningSecret(root, reference string) (*localSigningSecret, error) {
	root = strings.TrimSpace(root)
	if root == "" || !filepath.IsAbs(root) {
		return nil, errors.New("local Secret root must be an absolute path")
	}
	parsed, err := url.Parse(strings.TrimSpace(reference))
	if err != nil || strings.ToLower(parsed.Scheme) != "local-file" || parsed.User != nil ||
		(parsed.Host != "" && parsed.Host != "localhost") {
		return nil, errors.New("signing Secret must use a local-file reference without a remote host")
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("resolve local Secret root: %w", err)
	}
	relative := strings.TrimPrefix(filepath.Clean("/"+parsed.Path), "/")
	if relative == "" || relative == "." {
		return nil, errors.New("local signing Secret path is required")
	}
	candidate := filepath.Join(resolvedRoot, relative)
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return nil, fmt.Errorf("resolve local signing Secret: %w", err)
	}
	if resolved != resolvedRoot && !strings.HasPrefix(resolved, resolvedRoot+string(filepath.Separator)) {
		return nil, errors.New("local signing Secret escapes configured root")
	}
	info, err := os.Lstat(candidate)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("local signing Secret symlinks are forbidden")
	}
	resolvedInfo, err := os.Stat(resolved)
	if err != nil || !resolvedInfo.Mode().IsRegular() {
		return nil, errors.New("local signing Secret is not a regular file")
	}
	if resolvedInfo.Mode().Perm()&0o077 != 0 {
		return nil, errors.New("local signing Secret must not be accessible by group or others")
	}
	file, err := os.Open(resolved)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return decodeLocalSigningSecret(file)
}

func decodeLocalSigningSecret(reader io.Reader) (*localSigningSecret, error) {
	decoder := json.NewDecoder(io.LimitReader(reader, 64<<10))
	decoder.DisallowUnknownFields()
	var secret localSigningSecret
	if err := decoder.Decode(&secret); err != nil {
		return nil, errors.New("local signing Secret must be strict JSON")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, errors.New("local signing Secret contains trailing data")
	}
	if secret.KeystorePassword == "" || secret.KeyPassword == "" || len(secret.KeystorePassword) > 4096 || len(secret.KeyPassword) > 4096 {
		secret.erase()
		return nil, errors.New("local signing Secret is incomplete")
	}
	return &secret, nil
}

func (secret *localSigningSecret) erase() {
	if secret == nil {
		return
	}
	secret.KeystorePassword = ""
	secret.KeyPassword = ""
}

func nextAuth(current *state, stateDir string) map[string]any {
	authStateMu.Lock()
	defer authStateMu.Unlock()
	now := time.Now().UnixMilli()
	if now <= current.LastTimestamp {
		now = current.LastTimestamp + 1
	}
	current.LastTimestamp = now
	encoded, _ := json.MarshalIndent(current, "", "  ")
	_ = writePrivateFile(filepath.Join(stateDir, "state.json"), encoded)
	return map[string]any{"agent_id": current.AgentID, "nonce": newNonce(), "timestamp": now}
}

func mtlsClient(current *state) (*http.Client, error) {
	certificate, err := tls.LoadX509KeyPair(current.Certificate, current.PrivateKey)
	if err != nil {
		return nil, err
	}
	gatewayCA := current.GatewayCA
	if gatewayCA == "" {
		// Backward compatibility for state files created before gatewayCa existed.
		gatewayCA = current.ClientCA
	}
	caPEM, err := os.ReadFile(gatewayCA)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("Agent CA is invalid")
	}
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate}, RootCAs: pool}}
	return &http.Client{Transport: transport, Timeout: 45 * time.Second}, nil
}

func clientWithServerCA(path string) (*http.Client, error) {
	if strings.TrimSpace(path) == "" {
		return &http.Client{Timeout: 45 * time.Second}, nil
	}
	caPEM, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read control-plane CA: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("control-plane CA is invalid")
	}
	return &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    pool,
	}}, Timeout: 45 * time.Second}, nil
}

func validateLocalState(root string, current *state) error {
	if current == nil || current.AgentID <= 0 || current.Protocol <= 0 || strings.TrimSpace(current.GatewayURL) == "" {
		return errors.New("Agent state identity is incomplete")
	}
	gatewayURL, err := url.Parse(current.GatewayURL)
	if err != nil || gatewayURL.Scheme != "https" || gatewayURL.Host == "" {
		return errors.New("Agent gateway URL must use HTTPS")
	}
	for _, path := range []string{filepath.Join(root, "state.json"), current.Certificate, current.PrivateKey, current.ClientCA, current.GatewayCA} {
		if strings.TrimSpace(path) == "" {
			continue
		}
		if err := requirePrivateStateFile(root, path); err != nil {
			return err
		}
	}
	due, err := certificateExpiresWithin(current.Certificate, 0, time.Now())
	if err != nil {
		return err
	}
	if due {
		return errors.New("Agent certificate is expired")
	}
	return nil
}

func requirePrivateStateFile(root, path string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return errors.New("resolve Agent state directory failed")
	}
	path, err = filepath.Abs(path)
	if err != nil || (path != root && !strings.HasPrefix(path, root+string(filepath.Separator))) {
		return errors.New("Agent state file escapes state directory")
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return errors.New("Agent state files must be private regular files")
	}
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil || (resolvedPath != resolvedRoot && !strings.HasPrefix(resolvedPath, resolvedRoot+string(filepath.Separator))) {
		return errors.New("resolved Agent state file escapes state directory")
	}
	return nil
}

func readPrivateToken(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return "", errors.New("registration token file must be a private regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, 4097))
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(raw))
	if value == "" || len(value) > 4096 {
		return "", errors.New("registration token file is empty or too large")
	}
	return value, nil
}

func loadState(dir string) (state, error) {
	var result state
	file, err := os.Open(filepath.Join(dir, "state.json"))
	if err != nil {
		return result, err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return result, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return result, errors.New("Agent state contains trailing data")
	}
	return result, nil
}
func postJSON(ctx context.Context, client *http.Client, url string, input any, output any) error {
	body, err := json.Marshal(input)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("control plane returned %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	if output != nil && len(data) > 0 {
		return json.Unmarshal(data, output)
	}
	return nil
}

func postAuthenticatedJSON(ctx context.Context, client *http.Client, current *state, stateDir, endpoint string, payload map[string]any, output any) error {
	authenticatedRequest.Lock()
	defer authenticatedRequest.Unlock()
	if payload == nil {
		payload = map[string]any{}
	}
	payload["auth"] = nextAuth(current, stateDir)
	return postJSON(ctx, client, endpoint, payload, output)
}

func writePrivateFile(path string, data []byte) error {
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return err
	}
	if err := os.Chmod(temporary, 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}
func newNonce() string {
	raw := make([]byte, 16)
	_, _ = rand.Read(raw)
	return hex.EncodeToString(raw)
}
func digestBytes(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func wait(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
func defaultStateDir() string {
	if value := os.Getenv("APPFORGE_LOCAL_AGENT_STATE"); value != "" {
		return value
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return ".appforge-local-agent"
	}
	return filepath.Join(dir, "appforge-local-agent")
}

func offlineSignCommand(args []string) error {
	flags := flag.NewFlagSet("offline-sign", flag.ContinueOnError)
	keyPath := flags.String("key", "", "ECDSA private key PEM")
	input := flags.String("input", "", "offline package path")
	signature := flags.String("signature", "", "signature output path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	key, err := loadECDSAKey(*keyPath)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(*input)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(data)
	r, s, err := ecdsa.Sign(rand.Reader, key, digest[:])
	if err != nil {
		return err
	}
	encoded, _ := json.Marshal(map[string]string{"sha256": hex.EncodeToString(digest[:]), "r": r.Text(16), "s": s.Text(16)})
	return writePrivateFile(*signature, encoded)
}
func offlineVerifyCommand(args []string) error {
	flags := flag.NewFlagSet("offline-verify", flag.ContinueOnError)
	certPath := flags.String("certificate", "", "Agent certificate PEM")
	input := flags.String("input", "", "offline package path")
	signature := flags.String("signature", "", "signature path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	certPEM, err := os.ReadFile(*certPath)
	if err != nil {
		return err
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return errors.New("certificate PEM is invalid")
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}
	publicKey, ok := certificate.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return errors.New("certificate does not contain an ECDSA key")
	}
	data, err := os.ReadFile(*input)
	if err != nil {
		return err
	}
	var signed map[string]string
	raw, err := os.ReadFile(*signature)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, &signed); err != nil {
		return err
	}
	digest := sha256.Sum256(data)
	if signed["sha256"] != hex.EncodeToString(digest[:]) {
		return errors.New("offline package SHA-256 mismatch")
	}
	r := new(big.Int)
	s := new(big.Int)
	r.SetString(signed["r"], 16)
	s.SetString(signed["s"], 16)
	if !ecdsa.Verify(publicKey, digest[:], r, s) {
		return errors.New("offline package signature is invalid")
	}
	fmt.Println("offline package signature verified")
	return nil
}
func loadECDSAKey(path string) (*ecdsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("private key PEM is invalid")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not ECDSA")
	}
	return key, nil
}
