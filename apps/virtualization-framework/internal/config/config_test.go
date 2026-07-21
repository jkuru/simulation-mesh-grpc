package config_test

import (
	"testing"

	"github.com/servicemesh/virtualization-framework/internal/config"
)

func TestFromEnv_Defaults(t *testing.T) {
	cfg := config.FromEnv()
	if cfg.Environment != "dev" {
		t.Fatalf("env = %q", cfg.Environment)
	}
	if cfg.IsProd() {
		t.Fatal("dev should not be prod")
	}
}

func TestIsProd(t *testing.T) {
	t.Setenv("ENVIRONMENT", "prod")
	cfg := config.FromEnv()
	if !cfg.IsProd() {
		t.Fatal("expected prod")
	}
}
