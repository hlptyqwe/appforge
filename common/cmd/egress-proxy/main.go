package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"appforge/common/egressproxy"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("egress proxy stopped", "error", err.Error())
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	allowlistPath := strings.TrimSpace(os.Getenv("APPFORGE_EGRESS_PROXY_ALLOWLIST_FILE"))
	if allowlistPath == "" {
		return fmt.Errorf("APPFORGE_EGRESS_PROXY_ALLOWLIST_FILE is required")
	}
	allowlistFile, err := os.Open(allowlistPath)
	if err != nil {
		return fmt.Errorf("open egress proxy allowlist: %w", err)
	}
	allowlist, err := egressproxy.ParseAllowlist(allowlistFile)
	_ = allowlistFile.Close()
	if err != nil {
		return err
	}
	maximumConnections := 256
	if raw := strings.TrimSpace(os.Getenv("APPFORGE_EGRESS_PROXY_MAX_CONNECTIONS")); raw != "" {
		maximumConnections, err = strconv.Atoi(raw)
		if err != nil {
			return fmt.Errorf("APPFORGE_EGRESS_PROXY_MAX_CONNECTIONS must be an integer")
		}
	}
	handler, err := egressproxy.NewHandler(allowlist, maximumConnections, 10*time.Second, 10*time.Minute)
	if err != nil {
		return err
	}
	address := strings.TrimSpace(os.Getenv("APPFORGE_EGRESS_PROXY_LISTEN"))
	if address == "" {
		address = ":3128"
	}
	server := &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.ListenAndServe() }()
	logger.Info("egress proxy started", "listen", address, "maximumConnections", maximumConnections)

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-shutdown:
	case serveErr := <-serverErrors:
		if serveErr != nil && serveErr != http.ErrServerClosed {
			return fmt.Errorf("serve egress proxy: %w", serveErr)
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown egress proxy: %w", err)
	}
	logger.Info("egress proxy stopped")
	return nil
}
