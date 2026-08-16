// Package siem exports already-redacted AppForge audit events to a customer
// SIEM endpoint without putting external availability on the API request path.
package siem

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

type Config struct {
	Enabled         bool          `json:"Enabled" yaml:"Enabled"`
	Endpoint        string        `json:"Endpoint" yaml:"Endpoint"`
	BearerTokenFile string        `json:"BearerTokenFile" yaml:"BearerTokenFile"`
	CACertificate   string        `json:"CACertificate" yaml:"CACertificate"`
	Timeout         time.Duration `json:"Timeout" yaml:"Timeout"`
	QueueSize       int           `json:"QueueSize" yaml:"QueueSize"`
	MaxAttempts     int           `json:"MaxAttempts" yaml:"MaxAttempts"`
	RetryBackoff    time.Duration `json:"RetryBackoff" yaml:"RetryBackoff"`
	AllowHTTP       bool          `json:"AllowHTTP" yaml:"AllowHTTP"`
}

type Event struct {
	Timestamp int64  `json:"timestamp"`
	TenantID  int64  `json:"tenantId"`
	UserID    int64  `json:"userId"`
	Username  string `json:"username"`
	Module    string `json:"module"`
	Action    string `json:"action"`
	Method    string `json:"method"`
	Path      string `json:"path"`
	Request   string `json:"request"`
	Response  string `json:"response"`
	IP        string `json:"ip"`
	CostMS    int64  `json:"costMs"`
}

type Exporter struct {
	config  Config
	client  *http.Client
	queue   chan Event
	dropped atomic.Uint64
	failed  atomic.Uint64
}

func New(config Config) (*Exporter, error) {
	if !config.Enabled {
		return nil, nil
	}
	endpoint, err := url.Parse(config.Endpoint)
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "https" && !(config.AllowHTTP && endpoint.Scheme == "http")) {
		return nil, errors.New("SIEM endpoint must be an absolute HTTPS URL")
	}
	if config.QueueSize <= 0 || config.QueueSize > 100000 {
		config.QueueSize = 1000
	}
	if config.Timeout <= 0 {
		config.Timeout = 10 * time.Second
	}
	if config.MaxAttempts <= 0 || config.MaxAttempts > 20 {
		config.MaxAttempts = 5
	}
	if config.RetryBackoff <= 0 {
		config.RetryBackoff = 500 * time.Millisecond
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if strings.TrimSpace(config.CACertificate) != "" {
		raw, err := os.ReadFile(config.CACertificate)
		if err != nil {
			return nil, fmt.Errorf("read SIEM CA certificate: %w", err)
		}
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(raw) {
			return nil, errors.New("SIEM CA certificate is invalid")
		}
		tlsConfig.RootCAs = pool
	}
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment, TLSClientConfig: tlsConfig, MaxIdleConns: 10, IdleConnTimeout: 30 * time.Second}
	return &Exporter{config: config, client: &http.Client{Transport: transport, Timeout: config.Timeout}, queue: make(chan Event, config.QueueSize)}, nil
}

func (exporter *Exporter) Start(ctx context.Context) {
	if exporter == nil {
		return
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-exporter.queue:
				if exporter.sendWithRetry(ctx, event) != nil {
					exporter.failed.Add(1)
				}
			}
		}
	}()
}

// Export queues an event without blocking a customer request. False means the
// bounded queue was full and the caller should emit an internal metric/log.
func (exporter *Exporter) Export(event Event) bool {
	if exporter == nil {
		return true
	}
	select {
	case exporter.queue <- event:
		return true
	default:
		exporter.dropped.Add(1)
		return false
	}
}

func (exporter *Exporter) Dropped() uint64 {
	if exporter == nil {
		return 0
	}
	return exporter.dropped.Load()
}

// Failed returns events that exhausted the configured delivery attempts.
func (exporter *Exporter) Failed() uint64 {
	if exporter == nil {
		return 0
	}
	return exporter.failed.Load()
}

func (exporter *Exporter) sendWithRetry(ctx context.Context, event Event) error {
	var lastErr error
	for attempt := 1; attempt <= exporter.config.MaxAttempts; attempt++ {
		if lastErr = exporter.send(ctx, event); lastErr == nil {
			return nil
		}
		if attempt == exporter.config.MaxAttempts {
			break
		}
		delay := exporter.config.RetryBackoff * time.Duration(1<<min(attempt-1, 6))
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return lastErr
}

func (exporter *Exporter) send(ctx context.Context, event Event) error {
	raw, err := json.Marshal(event)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, exporter.config.Endpoint, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "appforge-siem-exporter/1")
	if strings.TrimSpace(exporter.config.BearerTokenFile) != "" {
		token, err := os.ReadFile(exporter.config.BearerTokenFile)
		if err != nil {
			return fmt.Errorf("read SIEM bearer token: %w", err)
		}
		request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(token)))
		for index := range token {
			token[index] = 0
		}
	}
	response, err := exporter.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("SIEM endpoint returned %s", response.Status)
	}
	return nil
}
