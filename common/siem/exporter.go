// Package siem exports already-redacted AppForge audit events to a customer
// SIEM endpoint without putting external availability on the API request path.
package siem

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const (
	transportHTTPS     = "https"
	transportSyslogTLS = "syslog+tls"
	defaultSyslogPRI   = 134 // local0.info
	defaultSyslogApp   = "appforge"
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
	SyslogHostname  string        `json:"SyslogHostname" yaml:"SyslogHostname"`
	SyslogAppName   string        `json:"SyslogAppName" yaml:"SyslogAppName"`
	MaxMessageBytes int           `json:"MaxMessageBytes" yaml:"MaxMessageBytes"`
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
	mode    string
	tls     *tls.Config
	address string
	proxy   *url.URL
	host    string
	appName string
	queue   chan Event
	dropped atomic.Uint64
	failed  atomic.Uint64
}

func New(config Config) (*Exporter, error) {
	if !config.Enabled {
		return nil, nil
	}
	endpoint, err := url.Parse(config.Endpoint)
	if err != nil || endpoint.Host == "" || endpoint.User != nil || endpoint.Fragment != "" {
		return nil, errors.New("SIEM endpoint must be an absolute HTTPS or syslog+tls URL without credentials or fragment")
	}
	mode := endpoint.Scheme
	if mode != transportHTTPS && !(config.AllowHTTP && mode == "http") && mode != transportSyslogTLS {
		return nil, errors.New("SIEM endpoint must use HTTPS or syslog+tls")
	}
	if mode == transportSyslogTLS {
		if endpoint.Path != "" || endpoint.RawQuery != "" {
			return nil, errors.New("syslog+tls endpoint must not contain path or query")
		}
		host, port, splitErr := net.SplitHostPort(endpoint.Host)
		portNumber, portErr := strconv.Atoi(port)
		if splitErr != nil || strings.TrimSpace(host) == "" || portErr != nil || portNumber < 1 || portNumber > 65535 {
			return nil, errors.New("syslog+tls endpoint must contain an explicit valid host and port")
		}
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
	if config.MaxMessageBytes == 0 {
		config.MaxMessageBytes = 64 << 10
	}
	if config.MaxMessageBytes < 1024 || config.MaxMessageBytes > 1<<20 {
		return nil, errors.New("SIEM max message bytes must be between 1024 and 1048576")
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
	exporter := &Exporter{config: config, mode: mode, tls: tlsConfig, queue: make(chan Event, config.QueueSize)}
	if mode == transportSyslogTLS {
		tlsConfig.ServerName = endpoint.Hostname()
		exporter.address = endpoint.Host
		proxyRequest := &http.Request{URL: &url.URL{Scheme: "https", Host: endpoint.Host}}
		exporter.proxy, err = http.ProxyFromEnvironment(proxyRequest)
		if err != nil {
			return nil, fmt.Errorf("resolve SIEM syslog TLS proxy: %w", err)
		}
		if err = validateSyslogProxy(exporter.proxy); err != nil {
			return nil, err
		}
		exporter.host = sanitizeSyslogHeader(config.SyslogHostname, 255)
		if exporter.host == "-" {
			hostname, hostnameErr := os.Hostname()
			if hostnameErr == nil {
				exporter.host = sanitizeSyslogHeader(hostname, 255)
			}
		}
		exporter.appName = sanitizeSyslogHeader(config.SyslogAppName, 48)
		if exporter.appName == "-" {
			exporter.appName = defaultSyslogApp
		}
		return exporter, nil
	}
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment, TLSClientConfig: tlsConfig, MaxIdleConns: 10, IdleConnTimeout: 30 * time.Second}
	exporter.client = &http.Client{Transport: transport, Timeout: config.Timeout}
	return exporter, nil
}

func validateSyslogProxy(proxy *url.URL) error {
	if proxy != nil && (proxy.Scheme != "http" || proxy.Host == "" || proxy.User != nil) {
		return errors.New("SIEM syslog TLS proxy must be an absolute HTTP URL without credentials")
	}
	return nil
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
	if exporter.mode == transportSyslogTLS {
		return exporter.sendSyslogTLS(ctx, event)
	}
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

func (exporter *Exporter) sendSyslogTLS(ctx context.Context, event Event) error {
	message, err := exporter.rfc5424Message(event)
	if err != nil {
		return err
	}
	frame := append([]byte(strconv.Itoa(len(message))+" "), message...)
	operationCtx, cancel := context.WithTimeout(ctx, exporter.config.Timeout)
	defer cancel()
	connection, err := exporter.dialSyslogTLS(operationCtx)
	if err != nil {
		return fmt.Errorf("connect SIEM syslog TLS endpoint: %w", err)
	}
	defer connection.Close()
	deadline := time.Now().Add(exporter.config.Timeout)
	if contextDeadline, ok := operationCtx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	if err = connection.SetDeadline(deadline); err != nil {
		return fmt.Errorf("set SIEM syslog TLS deadline: %w", err)
	}
	for written := 0; written < len(frame); {
		count, writeErr := connection.Write(frame[written:])
		if writeErr != nil {
			return fmt.Errorf("write SIEM syslog TLS frame: %w", writeErr)
		}
		if count <= 0 {
			return errors.New("write SIEM syslog TLS frame made no progress")
		}
		written += count
	}
	return nil
}

func (exporter *Exporter) dialSyslogTLS(ctx context.Context) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: exporter.config.Timeout, KeepAlive: 30 * time.Second}
	address := exporter.address
	if exporter.proxy != nil {
		address = exporter.proxy.Host
		if _, _, err := net.SplitHostPort(address); err != nil {
			address = net.JoinHostPort(address, "80")
		}
	}
	connection, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(exporter.config.Timeout)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	if err = connection.SetDeadline(deadline); err != nil {
		_ = connection.Close()
		return nil, err
	}
	if exporter.proxy != nil {
		connectRequest := "CONNECT " + exporter.address + " HTTP/1.1\r\nHost: " + exporter.address +
			"\r\nUser-Agent: appforge-siem-exporter/1\r\nProxy-Connection: Keep-Alive\r\n\r\n"
		if _, err = io.WriteString(connection, connectRequest); err != nil {
			_ = connection.Close()
			return nil, fmt.Errorf("write SIEM syslog CONNECT request: %w", err)
		}
		response, responseErr := http.ReadResponse(bufio.NewReader(connection), &http.Request{Method: http.MethodConnect})
		if responseErr != nil {
			_ = connection.Close()
			return nil, fmt.Errorf("read SIEM syslog CONNECT response: %w", responseErr)
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			if response.Body != nil {
				_ = response.Body.Close()
			}
			_ = connection.Close()
			return nil, fmt.Errorf("SIEM syslog CONNECT proxy returned %s", response.Status)
		}
	}
	tlsConnection := tls.Client(connection, exporter.tls.Clone())
	if err = tlsConnection.HandshakeContext(ctx); err != nil {
		_ = connection.Close()
		return nil, err
	}
	return tlsConnection, nil
}

func (exporter *Exporter) rfc5424Message(event Event) ([]byte, error) {
	payload, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}
	timestamp := time.Now().UTC()
	if event.Timestamp > 0 {
		timestamp = time.UnixMilli(event.Timestamp).UTC()
	}
	structuredData := fmt.Sprintf(
		`[appforge@32473 tenantId="%d" userId="%d" module="%s" action="%s"]`,
		event.TenantID, event.UserID, escapeSyslogParam(event.Module), escapeSyslogParam(event.Action),
	)
	message := []byte(fmt.Sprintf(
		"<%d>1 %s %s %s - AUDIT %s %s",
		defaultSyslogPRI, timestamp.Format(time.RFC3339Nano), exporter.host, exporter.appName, structuredData, payload,
	))
	if len(message) > exporter.config.MaxMessageBytes {
		return nil, fmt.Errorf("SIEM RFC5424 message exceeds configured limit: %d > %d", len(message), exporter.config.MaxMessageBytes)
	}
	return message, nil
}

func sanitizeSyslogHeader(value string, maximum int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	var builder strings.Builder
	for _, character := range value {
		if character >= 33 && character <= 126 && character != '=' && character != ']' && character != '"' {
			builder.WriteRune(character)
		} else {
			builder.WriteByte('_')
		}
		if builder.Len() >= maximum {
			break
		}
	}
	if builder.Len() == 0 {
		return "-"
	}
	return builder.String()
}

func escapeSyslogParam(value string) string {
	var builder strings.Builder
	for _, character := range value {
		switch character {
		case '\\', '"', ']':
			builder.WriteByte('\\')
			builder.WriteRune(character)
		default:
			if character < 32 || character == 127 {
				builder.WriteByte('_')
			} else {
				builder.WriteRune(character)
			}
		}
		if builder.Len() >= 256 {
			break
		}
	}
	return builder.String()
}
