package egressproxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	defaultMaximumConnections = 256
	defaultDialTimeout        = 10 * time.Second
	defaultConnectionTimeout  = 10 * time.Minute
)

// Handler is an HTTPS CONNECT-only forward proxy. It resolves a permitted
// hostname once and dials the resolved address directly, avoiding a second DNS
// lookup between authorization and connection.
type Handler struct {
	allowlist         *Allowlist
	resolver          *net.Resolver
	dialer            *net.Dialer
	connectionLimit   chan struct{}
	connectionTimeout time.Duration
}

// NewHandler creates a bounded CONNECT proxy handler.
func NewHandler(allowlist *Allowlist, maximumConnections int, dialTimeout, connectionTimeout time.Duration) (*Handler, error) {
	if allowlist == nil || len(allowlist.rules) == 0 {
		return nil, errors.New("egress proxy allowlist is required")
	}
	if maximumConnections <= 0 {
		maximumConnections = defaultMaximumConnections
	}
	if maximumConnections > 10000 {
		return nil, errors.New("egress proxy maximum connections exceeds 10000")
	}
	if dialTimeout <= 0 {
		dialTimeout = defaultDialTimeout
	}
	if connectionTimeout <= 0 {
		connectionTimeout = defaultConnectionTimeout
	}
	return &Handler{
		allowlist:         allowlist,
		resolver:          net.DefaultResolver,
		dialer:            &net.Dialer{Timeout: dialTimeout, KeepAlive: 30 * time.Second},
		connectionLimit:   make(chan struct{}, maximumConnections),
		connectionTimeout: connectionTimeout,
	}, nil
}

func (handler *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodGet && (request.URL.Path == "/healthz" || request.URL.Path == "/readyz") && request.URL.Host == "" {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"status":"ok"}`))
		return
	}
	if request.Method != http.MethodConnect {
		http.Error(writer, "only HTTPS CONNECT is supported", http.StatusMethodNotAllowed)
		return
	}
	host, port, err := net.SplitHostPort(request.Host)
	if err != nil || !handler.allowlist.Allows(host, port) {
		http.Error(writer, "destination is not allowlisted", http.StatusForbidden)
		return
	}
	select {
	case handler.connectionLimit <- struct{}{}:
		defer func() { <-handler.connectionLimit }()
	default:
		http.Error(writer, "connection limit reached", http.StatusServiceUnavailable)
		return
	}

	upstream, err := handler.dialApproved(request.Context(), host, port)
	if err != nil {
		http.Error(writer, "approved destination is unavailable", http.StatusBadGateway)
		return
	}
	defer upstream.Close()

	hijacker, ok := writer.(http.Hijacker)
	if !ok {
		http.Error(writer, "connection hijacking is unavailable", http.StatusInternalServerError)
		return
	}
	client, buffered, err := hijacker.Hijack()
	if err != nil {
		return
	}
	defer client.Close()
	deadline := time.Now().Add(handler.connectionTimeout)
	_ = client.SetDeadline(deadline)
	_ = upstream.SetDeadline(deadline)
	if _, err := buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}
	if err := buffered.Flush(); err != nil {
		return
	}

	completed := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(upstream, buffered)
		closeWrite(upstream)
		completed <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(client, upstream)
		closeWrite(client)
		completed <- struct{}{}
	}()
	<-completed
}

func (handler *Handler) dialApproved(ctx context.Context, host, port string) (net.Conn, error) {
	if address := net.ParseIP(strings.Trim(host, "[]")); address != nil {
		return handler.dialer.DialContext(ctx, "tcp", net.JoinHostPort(address.String(), port))
	}
	addresses, err := handler.resolver.LookupIPAddr(ctx, host)
	if err != nil || len(addresses) == 0 {
		return nil, errors.New("resolve approved destination failed")
	}
	var dialErrors []error
	for _, address := range addresses {
		connection, dialErr := handler.dialer.DialContext(ctx, "tcp", net.JoinHostPort(address.IP.String(), port))
		if dialErr == nil {
			return connection, nil
		}
		dialErrors = append(dialErrors, dialErr)
	}
	return nil, fmt.Errorf("dial approved destination: %w", errors.Join(dialErrors...))
}

func closeWrite(connection net.Conn) {
	if tcp, ok := connection.(*net.TCPConn); ok {
		_ = tcp.CloseWrite()
	}
}
