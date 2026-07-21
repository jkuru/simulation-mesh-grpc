// Package contract is the shared vocabulary for header-driven service virtualization.
//
// Both the sample app (reference-app) and the platform (virtualization-framework)
// import these constants so the product surface cannot drift.
package contract

// Simulation request header — value is a scenario name (e.g. "fraud-declined").
const SimulationHeader = "test-data-simulation-action-name"

// MicrocksOperationHeader is the rewrite target used by real Microcks (and
// teaching microcks-mock) to select an operation/example.
const MicrocksOperationHeader = "x-microcks-operation"

// Workload label that opts a Deployment into EnvoyFilter capture/inject.
const (
	PropagationLabelKey   = "simulation.io/propagation"
	PropagationLabelValue = "enabled"
)

// Operator-managed object labels.
const (
	ManagedByLabelKey   = "app.kubernetes.io/managed-by"
	ManagedByLabelValue = "virtualization-framework"
	ManifestLabelKey    = "simulation.io/manifest"
)

// Virtual backend strategies (SimulationManifest / platform config).
const (
	// BackendTeachingMock is the in-repo microcks-mock gRPC server (default).
	BackendTeachingMock = "teaching-mock"
	// BackendMicrocks is a real Microcks (or compatible) virtual service.
	BackendMicrocks = "microcks"
)

// Default in-cluster virtual backend host:port (teaching path).
const DefaultMicrocksHostPort = "microcks-svc.simulation-system.svc.cluster.local:9090"

// SystemNamespace hosts the operator and shared virtual backend.
const SystemNamespace = "simulation-system"

// Environment label/annotation key for prod guardrails.
const EnvironmentKey = "simulation.io/environment"
