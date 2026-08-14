package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	exitUsage   = 2
	exitAuth    = 3
	exitAPI     = 5
	exitTimeout = 6
)

type config struct {
	BaseURL string `json:"baseUrl"`
	APIKey  string `json:"apiKey"`
}

type options struct {
	JSON    bool
	Timeout time.Duration
	Retries int
	Config  string
}

type client struct {
	baseURL string
	apiKey  string
	http    *http.Client
	retries int
}

type apiError struct {
	Status  int
	Code    string
	Message string
}

func (e *apiError) Error() string {
	if e.Code == "" {
		return e.Message
	}
	return e.Code + ": " + e.Message
}

type envelope struct {
	Code       int             `json:"code"`
	Msg        string          `json:"msg"`
	Total      int64           `json:"total"`
	HasNext    bool            `json:"hasNext"`
	NextCursor int64           `json:"nextCursor"`
	Data       json.RawMessage `json:"data"`
}

type application struct {
	ID          int64  `json:"id"`
	AppCode     string `json:"appCode"`
	AppName     string `json:"appName"`
	PackageName string `json:"packageName"`
	Status      int32  `json:"status"`
}

type uploadTicket struct {
	ObjectID  int64  `json:"objectId"`
	UploadURL string `json:"uploadUrl"`
	ExpiresAt int64  `json:"expiresAt"`
}

type storageObject struct {
	ObjectID  int64  `json:"objectId"`
	AppID     int64  `json:"appId"`
	SHA256    string `json:"sha256"`
	SizeBytes int64  `json:"sizeBytes"`
	Status    int32  `json:"status"`
}

type version struct {
	ID                int64  `json:"id"`
	AppID             int64  `json:"appId"`
	VersionCode       int64  `json:"versionCode"`
	VersionName       string `json:"versionName"`
	SourceAPKObjectID int64  `json:"sourceApkObjectId"`
	SourceAPKSHA256   string `json:"sourceApkSha256"`
	Status            int32  `json:"status"`
}

type build struct {
	ID           int64  `json:"id"`
	AppID        int64  `json:"appId"`
	VersionID    int64  `json:"versionId"`
	ChannelID    int64  `json:"channelId"`
	Status       int32  `json:"status"`
	APKObjectID  int64  `json:"apkObjectId"`
	APKURL       string `json:"apkUrl"`
	ErrorMessage string `json:"errorMessage"`
}

type artifactDownload struct {
	DownloadURL string `json:"downloadUrl"`
	ExpiresAt   int64  `json:"expiresAt"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	defaults, err := defaultConfigPath()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitUsage
	}
	global := flag.NewFlagSet("appforgectl", flag.ContinueOnError)
	global.SetOutput(stderr)
	opt := options{}
	global.BoolVar(&opt.JSON, "json", false, "print machine-readable JSON")
	global.DurationVar(&opt.Timeout, "timeout", 10*time.Minute, "command timeout")
	global.IntVar(&opt.Retries, "retries", 3, "retry count for transient failures")
	global.StringVar(&opt.Config, "config", envOr("APPFORGE_CONFIG", defaults), "configuration file")
	if err := global.Parse(args); err != nil {
		return exitUsage
	}
	remaining := global.Args()
	if len(remaining) < 2 {
		printUsage(stderr)
		return exitUsage
	}
	if remaining[0] == "auth" && remaining[1] == "configure" {
		return configureCommand(remaining[2:], opt, stdout, stderr)
	}
	cfg, err := loadRuntimeConfig(opt.Config)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitAuth
	}
	if cfg.BaseURL == "" || cfg.APIKey == "" {
		fmt.Fprintln(stderr, "base URL and API key are required; run appforgectl auth configure or set APPFORGE_BASE_URL and APPFORGE_API_KEY")
		return exitAuth
	}
	ctx, cancel := context.WithTimeout(context.Background(), opt.Timeout)
	defer cancel()
	api := &client{baseURL: strings.TrimRight(cfg.BaseURL, "/"), apiKey: cfg.APIKey,
		http: &http.Client{Timeout: min(opt.Timeout, 2*time.Minute)}, retries: max(opt.Retries, 0)}

	var commandErr error
	switch remaining[0] + " " + remaining[1] {
	case "app list":
		commandErr = appList(ctx, api, remaining[2:], opt.JSON, stdout)
	case "version upload":
		commandErr = versionUpload(ctx, api, remaining[2:], opt.JSON, stdout)
	case "build create":
		commandErr = buildCreate(ctx, api, remaining[2:], opt.JSON, stdout)
	case "build wait":
		commandErr = buildWait(ctx, api, remaining[2:], opt.JSON, stdout)
	case "artifact download":
		commandErr = artifactDownloadCommand(ctx, api, remaining[2:], opt.JSON, stdout)
	default:
		printUsage(stderr)
		return exitUsage
	}
	if commandErr == nil {
		return 0
	}
	fmt.Fprintln(stderr, commandErr)
	if errors.Is(commandErr, context.DeadlineExceeded) {
		return exitTimeout
	}
	var apiErr *apiError
	if errors.As(commandErr, &apiErr) && apiErr.Status == http.StatusUnauthorized {
		return exitAuth
	}
	return exitAPI
}

func configureCommand(args []string, opt options, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("auth configure", flag.ContinueOnError)
	flags.SetOutput(stderr)
	baseURL := flags.String("base-url", envOr("APPFORGE_BASE_URL", ""), "Open API base URL")
	apiKey := flags.String("api-key", envOr("APPFORGE_API_KEY", ""), "API key")
	if flags.Parse(args) != nil || strings.TrimSpace(*baseURL) == "" || strings.TrimSpace(*apiKey) == "" {
		fmt.Fprintln(stderr, "--base-url and --api-key are required")
		return exitUsage
	}
	cfg := config{BaseURL: strings.TrimRight(strings.TrimSpace(*baseURL), "/"), APIKey: strings.TrimSpace(*apiKey)}
	if err := saveConfig(opt.Config, cfg); err != nil {
		fmt.Fprintln(stderr, err)
		return exitAPI
	}
	if opt.JSON {
		if err := writeJSON(stdout, map[string]any{"configured": true, "config": opt.Config}); err != nil {
			fmt.Fprintln(stderr, err)
			return exitAPI
		}
		return 0
	}
	fmt.Fprintf(stdout, "Configuration saved to %s (API key hidden)\n", opt.Config)
	return 0
}

func appList(ctx context.Context, api *client, args []string, jsonOutput bool, stdout io.Writer) error {
	flags := flag.NewFlagSet("app list", flag.ContinueOnError)
	keyword := flags.String("keyword", "", "application keyword")
	limit := flags.Int("limit", 100, "page size")
	if err := flags.Parse(args); err != nil {
		return err
	}
	path := "/open/v1/apps?limit=" + strconv.Itoa(*limit)
	if *keyword != "" {
		path += "&keyword=" + url.QueryEscape(*keyword)
	}
	var data []application
	response, err := api.get(ctx, path, &data)
	if err != nil {
		return err
	}
	if jsonOutput {
		return writeJSON(stdout, map[string]any{"data": data, "total": response.Total, "hasNext": response.HasNext, "nextCursor": response.NextCursor})
	}
	for _, item := range data {
		fmt.Fprintf(stdout, "%d\t%s\t%s\t%s\n", item.ID, item.AppCode, item.AppName, item.PackageName)
	}
	return nil
}

func versionUpload(ctx context.Context, api *client, args []string, jsonOutput bool, stdout io.Writer) error {
	flags := flag.NewFlagSet("version upload", flag.ContinueOnError)
	appID := flags.Int64("app-id", 0, "application ID")
	fileName := flags.String("file", "", "APK file")
	versionCode := flags.Int64("version-code", 0, "Android versionCode")
	versionName := flags.String("version-name", "", "Android versionName")
	releaseNotes := flags.String("release-notes", "", "release notes")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *appID <= 0 || *fileName == "" || *versionCode <= 0 || *versionName == "" {
		return errors.New("--app-id, --file, --version-code and --version-name are required")
	}
	file, err := os.Open(*fileName)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(info.Name())))
	if contentType == "" {
		contentType = "application/vnd.android.package-archive"
	}
	var ticket uploadTicket
	if _, err := api.mutate(ctx, http.MethodPost, "/open/v1/uploads", map[string]any{
		"appId": *appID, "objectType": 1, "fileName": info.Name(), "sizeBytes": info.Size(), "contentType": contentType,
	}, &ticket, newIdempotencyKey()); err != nil {
		return err
	}
	if err := uploadFile(ctx, api.http, ticket.UploadURL, contentType, info.Size(), file); err != nil {
		return err
	}
	var object storageObject
	if _, err := api.mutate(ctx, http.MethodPost, fmt.Sprintf("/open/v1/uploads/%d/complete", ticket.ObjectID), map[string]any{}, &object, newIdempotencyKey()); err != nil {
		return err
	}
	var created version
	if _, err := api.mutate(ctx, http.MethodPost, "/open/v1/versions", map[string]any{
		"appId": *appID, "versionCode": *versionCode, "versionName": *versionName,
		"sourceApkObjectId": object.ObjectID, "sourceApkSha256": digest, "releaseNotes": *releaseNotes,
	}, &created, newIdempotencyKey()); err != nil {
		return err
	}
	if jsonOutput {
		return writeJSON(stdout, created)
	}
	fmt.Fprintf(stdout, "Version %d created (APK object %d, sha256 %s)\n", created.ID, object.ObjectID, digest)
	return nil
}

func buildCreate(ctx context.Context, api *client, args []string, jsonOutput bool, stdout io.Writer) error {
	flags := flag.NewFlagSet("build create", flag.ContinueOnError)
	appID := flags.Int64("app-id", 0, "application ID")
	versionID := flags.Int64("version-id", 0, "version ID")
	channelID := flags.Int64("channel-id", 0, "channel ID")
	signingID := flags.Int64("signing-config-id", 0, "signing config ID")
	brandingID := flags.Int64("branding-profile-id", 0, "branding profile ID")
	productID := flags.Int64("white-label-product-id", 0, "white-label product ID")
	priority := flags.Int("priority", 0, "build priority")
	pool := flags.String("pool", "default", "builder pool")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *appID <= 0 || *versionID <= 0 || *channelID <= 0 || (*signingID <= 0 && *productID <= 0) {
		return errors.New("--app-id, --version-id, --channel-id and signing config or white-label product are required")
	}
	var created build
	_, err := api.mutate(ctx, http.MethodPost, "/open/v1/builds", map[string]any{
		"appId": *appID, "versionId": *versionID, "channelId": *channelID, "signingConfigId": *signingID,
		"brandingProfileId": *brandingID, "whiteLabelProductId": *productID, "priority": *priority, "poolCode": *pool,
	}, &created, newIdempotencyKey())
	if err != nil {
		return err
	}
	if jsonOutput {
		return writeJSON(stdout, created)
	}
	fmt.Fprintf(stdout, "Build %d queued\n", created.ID)
	return nil
}

func buildWait(ctx context.Context, api *client, args []string, jsonOutput bool, stdout io.Writer) error {
	flags := flag.NewFlagSet("build wait", flag.ContinueOnError)
	id := flags.Int64("id", 0, "build ID")
	interval := flags.Duration("interval", 2*time.Second, "poll interval")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *id <= 0 {
		return errors.New("--id is required")
	}
	for {
		var item build
		if _, err := api.get(ctx, fmt.Sprintf("/open/v1/builds/%d", *id), &item); err != nil {
			return err
		}
		switch item.Status {
		case 5:
			if jsonOutput {
				return writeJSON(stdout, item)
			}
			fmt.Fprintf(stdout, "Build %d succeeded; artifact object %d\n", item.ID, item.APKObjectID)
			return nil
		case 6, 7:
			return fmt.Errorf("build %d finished unsuccessfully: %s", item.ID, item.ErrorMessage)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(*interval):
		}
	}
}

func artifactDownloadCommand(ctx context.Context, api *client, args []string, jsonOutput bool, stdout io.Writer) error {
	flags := flag.NewFlagSet("artifact download", flag.ContinueOnError)
	id := flags.Int64("id", 0, "artifact object ID")
	output := flags.String("output", "channel.apk", "output file")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *id <= 0 || *output == "" {
		return errors.New("--id and --output are required")
	}
	var signed artifactDownload
	if _, err := api.get(ctx, fmt.Sprintf("/open/v1/artifacts/%d/download", *id), &signed); err != nil {
		return err
	}
	if err := downloadFile(ctx, api.http, signed.DownloadURL, *output); err != nil {
		return err
	}
	if jsonOutput {
		return writeJSON(stdout, map[string]any{"artifactId": *id, "output": *output, "expiresAt": signed.ExpiresAt})
	}
	fmt.Fprintf(stdout, "Artifact %d downloaded to %s\n", *id, *output)
	return nil
}

func (c *client) get(ctx context.Context, path string, target any) (*envelope, error) {
	return c.request(ctx, http.MethodGet, path, nil, target, "")
}

func (c *client) mutate(ctx context.Context, method, path string, payload, target any, idempotencyKey string) (*envelope, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return c.request(ctx, method, path, body, target, idempotencyKey)
}

func (c *client) request(ctx context.Context, method, path string, body []byte, target any, idempotencyKey string) (*envelope, error) {
	for attempt := 0; ; attempt++ {
		request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		request.Header.Set("Authorization", "Bearer "+c.apiKey)
		request.Header.Set("Accept", "application/json")
		if body != nil {
			request.Header.Set("Content-Type", "application/json")
		}
		if idempotencyKey != "" {
			request.Header.Set("Idempotency-Key", idempotencyKey)
		}
		response, err := c.http.Do(request)
		if err != nil {
			if attempt < c.retries && ctx.Err() == nil {
				if err := waitForRetry(ctx, time.Duration(1<<min(attempt, 6))*100*time.Millisecond); err != nil {
					return nil, err
				}
				continue
			}
			return nil, err
		}
		responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, 2<<20))
		_ = response.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if (response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500) && attempt < c.retries {
			if err := waitForRetry(ctx, retryDelay(response, attempt)); err != nil {
				return nil, err
			}
			continue
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return nil, decodeAPIError(response.StatusCode, responseBody)
		}
		var result envelope
		if err := json.Unmarshal(responseBody, &result); err != nil {
			return nil, fmt.Errorf("decode API response: %w", err)
		}
		if result.Code != 200 {
			return nil, &apiError{Status: response.StatusCode, Code: strconv.Itoa(result.Code), Message: result.Msg}
		}
		if target != nil && len(result.Data) > 0 && string(result.Data) != "null" {
			if err := json.Unmarshal(result.Data, target); err != nil {
				return nil, fmt.Errorf("decode API data: %w", err)
			}
		}
		return &result, nil
	}
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func retryDelay(response *http.Response, attempt int) time.Duration {
	if value, err := strconv.Atoi(response.Header.Get("Retry-After")); err == nil && value > 0 && value <= 60 {
		return time.Duration(value) * time.Second
	}
	return time.Duration(1<<min(attempt, 6)) * 200 * time.Millisecond
}

func decodeAPIError(statusCode int, body []byte) error {
	var value struct {
		Code    any    `json:"code"`
		Message string `json:"message"`
		Msg     string `json:"msg"`
	}
	_ = json.Unmarshal(body, &value)
	message := value.Message
	if message == "" {
		message = value.Msg
	}
	if message == "" {
		message = strings.TrimSpace(string(body))
	}
	return &apiError{Status: statusCode, Code: fmt.Sprint(value.Code), Message: message}
}

func uploadFile(ctx context.Context, httpClient *http.Client, target, contentType string, size int64, file *os.File) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, target, file)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", contentType)
	request.ContentLength = size
	response, err := httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("upload failed with HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func downloadFile(ctx context.Context, httpClient *http.Client, target, output string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("artifact download failed with HTTP %d", response.StatusCode)
	}
	temporary := output + ".part"
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, response.Body)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(temporary, output)
}

func defaultConfigPath() (string, error) {
	if value := strings.TrimSpace(os.Getenv("APPFORGE_CONFIG")); value != "" {
		return value, nil
	}
	directory, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, "appforge", "config.json"), nil
}

func loadConfig(path string) (config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return config{}, fmt.Errorf("configuration not found at %s", path)
		}
		return config{}, err
	}
	var result config
	if err := json.Unmarshal(data, &result); err != nil {
		return config{}, fmt.Errorf("decode configuration: %w", err)
	}
	return result, nil
}

func loadRuntimeConfig(path string) (config, error) {
	result := config{
		BaseURL: strings.TrimSpace(os.Getenv("APPFORGE_BASE_URL")),
		APIKey:  strings.TrimSpace(os.Getenv("APPFORGE_API_KEY")),
	}
	if result.BaseURL != "" && result.APIKey != "" {
		return result, nil
	}
	stored, err := loadConfig(path)
	if err != nil {
		return config{}, err
	}
	if result.BaseURL == "" {
		result.BaseURL = stored.BaseURL
	}
	if result.APIKey == "" {
		result.APIKey = stored.APIKey
	}
	return result, nil
}

func saveConfig(path string, value config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

func newIdempotencyKey() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "cli-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return "cli-" + hex.EncodeToString(value)
}

func writeJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("encode JSON output: %w", err)
	}
	return nil
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func printUsage(writer io.Writer) {
	fmt.Fprintln(writer, `Usage:
  appforgectl [--json] [--timeout 10m] auth configure --base-url URL --api-key KEY
  appforgectl [--json] app list [--keyword TEXT]
  appforgectl [--json] version upload --app-id ID --file APP.apk --version-code N --version-name NAME
  appforgectl [--json] build create --app-id ID --version-id ID --channel-id ID --signing-config-id ID
  appforgectl [--json] build wait --id ID
  appforgectl [--json] artifact download --id OBJECT_ID --output channel.apk`)
}
