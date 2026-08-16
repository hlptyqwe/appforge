package observability

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/zeromicro/go-zero/core/service"
)

const (
	defaultMetricsHost = "0.0.0.0"
	defaultMetricsPort = 9101
	defaultMetricsPath = "/metrics"
	defaultOTLPPath    = "/v1/traces"
)

// ApplyEnvironment applies deployment-owned observability settings to a
// go-zero service configuration. Secret OTLP headers remain in the protected
// runtime configuration and are never accepted through these environment
// variables.
func ApplyEnvironment(conf *service.ServiceConf) error {
	if conf == nil {
		return fmt.Errorf("service config is required")
	}
	if raw, ok := os.LookupEnv("APPFORGE_PROMETHEUS_ENABLED"); ok {
		enabled, err := strconv.ParseBool(strings.TrimSpace(raw))
		if err != nil {
			return fmt.Errorf("APPFORGE_PROMETHEUS_ENABLED must be a boolean: %w", err)
		}
		if !enabled {
			conf.Prometheus.Host = ""
		} else {
			conf.Prometheus.Host = defaultMetricsHost
			conf.Prometheus.Port = defaultMetricsPort
			conf.Prometheus.Path = defaultMetricsPath
			if value := strings.TrimSpace(os.Getenv("APPFORGE_PROMETHEUS_HOST")); value != "" {
				if net.ParseIP(value) == nil {
					return fmt.Errorf("APPFORGE_PROMETHEUS_HOST must be an IP address")
				}
				conf.Prometheus.Host = value
			}
			if value := strings.TrimSpace(os.Getenv("APPFORGE_PROMETHEUS_PORT")); value != "" {
				port, err := strconv.Atoi(value)
				if err != nil || port < 1 || port > 65535 {
					return fmt.Errorf("APPFORGE_PROMETHEUS_PORT must be between 1 and 65535")
				}
				conf.Prometheus.Port = port
			}
			if value := strings.TrimSpace(os.Getenv("APPFORGE_PROMETHEUS_PATH")); value != "" {
				if !validHTTPPath(value) {
					return fmt.Errorf("APPFORGE_PROMETHEUS_PATH must be an absolute path without query or fragment")
				}
				conf.Prometheus.Path = value
			}
		}
	}

	endpoint := strings.TrimSpace(os.Getenv("APPFORGE_OTLP_ENDPOINT"))
	if endpoint == "" {
		return nil
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("APPFORGE_OTLP_ENDPOINT must be an HTTP(S) URL without credentials, query, or fragment")
	}
	allowHTTP, err := optionalBoolean("APPFORGE_OTLP_ALLOW_HTTP", false)
	if err != nil {
		return err
	}
	switch parsed.Scheme {
	case "https":
		conf.Telemetry.OtlpHttpSecure = true
	case "http":
		if !allowHTTP {
			return fmt.Errorf("APPFORGE_OTLP_ENDPOINT must use HTTPS")
		}
		conf.Telemetry.OtlpHttpSecure = false
	default:
		return fmt.Errorf("APPFORGE_OTLP_ENDPOINT must use HTTP or HTTPS")
	}
	if parsed.Path == "" {
		parsed.Path = defaultOTLPPath
	}
	if !validHTTPPath(parsed.EscapedPath()) {
		return fmt.Errorf("APPFORGE_OTLP_ENDPOINT contains an invalid path")
	}
	if value := strings.TrimSpace(os.Getenv("APPFORGE_OTLP_SAMPLER")); value != "" {
		sampler, err := strconv.ParseFloat(value, 64)
		if err != nil || sampler < 0 || sampler > 1 {
			return fmt.Errorf("APPFORGE_OTLP_SAMPLER must be between 0 and 1")
		}
		conf.Telemetry.Sampler = sampler
	} else if conf.Telemetry.Sampler == 0 {
		conf.Telemetry.Sampler = 1
	}
	conf.Telemetry.Endpoint = parsed.Host
	conf.Telemetry.Batcher = "otlphttp"
	conf.Telemetry.OtlpHttpPath = parsed.EscapedPath()
	conf.Telemetry.Disabled = false
	return nil
}

func optionalBoolean(name string, fallback bool) (bool, error) {
	raw, ok := os.LookupEnv(name)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	value, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", name, err)
	}
	return value, nil
}

func validHTTPPath(value string) bool {
	return strings.HasPrefix(value, "/") && !strings.ContainsAny(value, "?#\r\n")
}
