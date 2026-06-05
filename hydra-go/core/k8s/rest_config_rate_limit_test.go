package k8s

import (
	"testing"

	"k8s.io/client-go/rest"
)

func TestApplyRESTConfigRateLimits_noop(t *testing.T) {
	var cfg rest.Config
	ApplyRESTConfigRateLimits(&cfg, 0, 0)
	if cfg.QPS != 0 || cfg.Burst != 0 {
		t.Fatalf("expected zero values unchanged, got QPS=%v Burst=%v", cfg.QPS, cfg.Burst)
	}
}

func TestApplyRESTConfigRateLimits_disable(t *testing.T) {
	var cfg rest.Config
	ApplyRESTConfigRateLimits(&cfg, -1, 0)
	if cfg.QPS >= 0 {
		t.Fatalf("expected negative QPS, got %v", cfg.QPS)
	}
}

func TestApplyRESTConfigRateLimits_positive(t *testing.T) {
	var cfg rest.Config
	ApplyRESTConfigRateLimits(&cfg, 100, 200)
	if cfg.QPS != 100 || cfg.Burst != 200 {
		t.Fatalf("expected QPS=100 Burst=200, got QPS=%v Burst=%v", cfg.QPS, cfg.Burst)
	}
}
