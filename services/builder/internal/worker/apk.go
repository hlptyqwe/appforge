package worker

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"
)

const channelAssetPath = "assets/appforge/channel.json"

type channelPayload struct {
	SchemaVersion int    `json:"schemaVersion"`
	TenantID      int64  `json:"tenantId"`
	AppID         int64  `json:"appId"`
	VersionID     int64  `json:"versionId"`
	BuildTaskID   int64  `json:"buildTaskId"`
	ChannelCode   string `json:"channelCode"`
	ChannelName   string `json:"channelName"`
	APIHost       string `json:"apiHost,omitempty"`
	LandingURL    string `json:"landingUrl,omitempty"`
	VersionCode   int64  `json:"versionCode"`
	VersionName   string `json:"versionName"`
	BuildTime     string `json:"buildTime"`
}

func injectChannelAsset(sourcePath, outputPath string, payload channelPayload) error {
	source, err := zip.OpenReader(sourcePath)
	if err != nil {
		return fmt.Errorf("open source APK: %w", err)
	}
	defer source.Close()

	output, err := os.OpenFile(outputPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create unsigned APK: %w", err)
	}
	writer := zip.NewWriter(output)
	closeWithError := func(cause error) error {
		_ = writer.Close()
		_ = output.Close()
		return cause
	}

	manifestFound := false
	for _, item := range source.File {
		cleanName := path.Clean(item.Name)
		if cleanName == "AndroidManifest.xml" {
			manifestFound = true
		}
		upperName := strings.ToUpper(cleanName)
		if cleanName == channelAssetPath || strings.HasPrefix(upperName, "META-INF/") {
			continue
		}
		header := item.FileHeader
		header.Name = cleanName
		header.SetModTime(time.Unix(0, 0).UTC())
		destination, err := writer.CreateHeader(&header)
		if err != nil {
			return closeWithError(fmt.Errorf("create APK entry %q: %w", cleanName, err))
		}
		input, err := item.Open()
		if err != nil {
			return closeWithError(fmt.Errorf("open APK entry %q: %w", cleanName, err))
		}
		_, copyErr := io.Copy(destination, input)
		closeErr := input.Close()
		if copyErr != nil {
			return closeWithError(fmt.Errorf("copy APK entry %q: %w", cleanName, copyErr))
		}
		if closeErr != nil {
			return closeWithError(fmt.Errorf("close APK entry %q: %w", cleanName, closeErr))
		}
	}
	if !manifestFound {
		return closeWithError(fmt.Errorf("source APK has no AndroidManifest.xml"))
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return closeWithError(fmt.Errorf("encode channel payload: %w", err))
	}
	assetHeader := &zip.FileHeader{Name: channelAssetPath, Method: zip.Deflate}
	assetHeader.SetMode(0o600)
	assetHeader.SetModTime(time.Unix(0, 0).UTC())
	asset, err := writer.CreateHeader(assetHeader)
	if err != nil {
		return closeWithError(fmt.Errorf("create channel asset: %w", err))
	}
	if _, err := asset.Write(encoded); err != nil {
		return closeWithError(fmt.Errorf("write channel asset: %w", err))
	}
	if err := writer.Close(); err != nil {
		_ = output.Close()
		return fmt.Errorf("finalize unsigned APK: %w", err)
	}
	if err := output.Close(); err != nil {
		return fmt.Errorf("close unsigned APK: %w", err)
	}
	return nil
}
