// Package config holds operator runtime settings (from env).
package config

import (
	"os"
	"strings"

	"github.com/servicemesh/virtualization-contract"
)

// Config is process-wide operator configuration.
type Config struct {
	// Environment is "dev" or "prod". prod forbids SimulationManifest reconcile.
	Environment string
	// SystemNamespace hosts shared Microcks / operator.
	SystemNamespace string
	// DefaultMicrocksHostPort is used when CR does not set microcksService.
	DefaultMicrocksHostPort string
	// SimulationHeader is the gRPC/HTTP2 metadata key.
	SimulationHeader string
	// PropagationLabel selects workloads for EnvoyFilters.
	PropagationLabelKey   string
	PropagationLabelValue string
	// MicrocksOperationHeader is used when virtualBackend=microcks.
	MicrocksOperationHeader string
}

// FromEnv loads config with defaults from the shared contract package.
func FromEnv() Config {
	return Config{
		Environment:             getenv("ENVIRONMENT", "dev"),
		SystemNamespace:         getenv("SYSTEM_NAMESPACE", contract.SystemNamespace),
		DefaultMicrocksHostPort: getenv("MICROCKS_SERVICE", contract.DefaultMicrocksHostPort),
		SimulationHeader:        getenv("SIMULATION_HEADER", contract.SimulationHeader),
		PropagationLabelKey:     getenv("PROPAGATION_LABEL_KEY", contract.PropagationLabelKey),
		PropagationLabelValue:   getenv("PROPAGATION_LABEL_VALUE", contract.PropagationLabelValue),
		MicrocksOperationHeader: getenv("MICROCKS_OPERATION_HEADER", contract.MicrocksOperationHeader),
	}
}

// IsProd returns true when environment is production.
func (c Config) IsProd() bool {
	return strings.EqualFold(c.Environment, "prod") || strings.EqualFold(c.Environment, "production")
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
