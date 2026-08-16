package observability

import (
	"testing"

	"github.com/zeromicro/go-zero/core/service"
)

func TestApplyEnvironment(t *testing.T) {
	t.Setenv("APPFORGE_PROMETHEUS_ENABLED", "true")
	t.Setenv("APPFORGE_PROMETHEUS_HOST", "127.0.0.1")
	t.Setenv("APPFORGE_PROMETHEUS_PORT", "19101")
	t.Setenv("APPFORGE_PROMETHEUS_PATH", "/internal/metrics")
	t.Setenv("APPFORGE_OTLP_ENDPOINT", "https://collector.example.com:4318/custom/traces")
	t.Setenv("APPFORGE_OTLP_SAMPLER", "0.25")
	var conf service.ServiceConf
	if err := ApplyEnvironment(&conf); err != nil {
		t.Fatal(err)
	}
	if conf.Prometheus.Host != "127.0.0.1" || conf.Prometheus.Port != 19101 || conf.Prometheus.Path != "/internal/metrics" {
		t.Fatalf("unexpected prometheus config: %+v", conf.Prometheus)
	}
	if conf.Telemetry.Endpoint != "collector.example.com:4318" || conf.Telemetry.Batcher != "otlphttp" ||
		conf.Telemetry.OtlpHttpPath != "/custom/traces" || !conf.Telemetry.OtlpHttpSecure || conf.Telemetry.Sampler != 0.25 {
		t.Fatalf("unexpected telemetry config: %+v", conf.Telemetry)
	}
}

func TestApplyEnvironmentRejectsInsecureAndInvalidValues(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "invalid metrics port", key: "APPFORGE_PROMETHEUS_PORT", value: "70000"},
		{name: "invalid metrics path", key: "APPFORGE_PROMETHEUS_PATH", value: "metrics"},
		{name: "insecure otlp", key: "APPFORGE_OTLP_ENDPOINT", value: "http://collector:4318/v1/traces"},
		{name: "otlp credentials", key: "APPFORGE_OTLP_ENDPOINT", value: "https://user:pass@collector:4318/v1/traces"},
		{name: "invalid sampler", key: "APPFORGE_OTLP_SAMPLER", value: "1.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("APPFORGE_PROMETHEUS_ENABLED", "true")
			t.Setenv(tt.key, tt.value)
			if tt.key == "APPFORGE_OTLP_SAMPLER" {
				t.Setenv("APPFORGE_OTLP_ENDPOINT", "https://collector:4318/v1/traces")
			}
			var conf service.ServiceConf
			if err := ApplyEnvironment(&conf); err == nil {
				t.Fatal("expected invalid observability environment to be rejected")
			}
		})
	}
}

func TestApplyEnvironmentAllowsExplicitLocalHTTP(t *testing.T) {
	t.Setenv("APPFORGE_OTLP_ENDPOINT", "http://127.0.0.1:4318/v1/traces")
	t.Setenv("APPFORGE_OTLP_ALLOW_HTTP", "true")
	var conf service.ServiceConf
	if err := ApplyEnvironment(&conf); err != nil {
		t.Fatal(err)
	}
	if conf.Telemetry.OtlpHttpSecure {
		t.Fatal("local HTTP acceptance endpoint must remain insecure")
	}
}
