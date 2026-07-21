package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SimulationManifestSpec defines the desired virtualization configuration.
type SimulationManifestSpec struct {
	// ThirdParties are external gRPC hosts to virtualize when the simulation header is present.
	// +kubebuilder:validation:MinItems=1
	ThirdParties []ThirdParty `json:"thirdParties"`

	// Scenarios are named response sets loaded into the virtual backend (documented / future Microcks).
	// +kubebuilder:validation:MinItems=1
	Scenarios []Scenario `json:"scenarios"`

	// MicrocksService is the in-cluster virtual backend host:port for VirtualService routes.
	// Defaults via mutating webhook to microcks-svc.simulation-system.svc.cluster.local:9090.
	// +optional
	MicrocksService string `json:"microcksService,omitempty"`

	// VirtualBackend selects the virtualization backend strategy:
	// teaching-mock (default, in-repo microcks-mock) or microcks (real Microcks-compatible).
	// When microcks, the operator also emits an EnvoyFilter to rewrite the simulation
	// header into x-microcks-operation on the virtual backend.
	// +kubebuilder:validation:Enum=teaching-mock;microcks
	// +kubebuilder:default=teaching-mock
	// +optional
	VirtualBackend string `json:"virtualBackend,omitempty"`
}

// ThirdParty describes one external dependency.
type ThirdParty struct {
	// Name is a short identifier used in scenarios (e.g. external-risk).
	Name string `json:"name"`
	// Host is the DNS name applications dial (e.g. external-risk-api.com).
	Host string `json:"host"`
	// Port is the destination port applications use.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`
	// Proto is an optional inline protobuf definition (for Microcks / docs).
	// +optional
	Proto string `json:"proto,omitempty"`
	// BackendHost is an optional in-cluster host for the "real" path when not using public DNS
	// (teaching clusters). If set, ServiceEntry endpoints point here.
	// +optional
	BackendHost string `json:"backendHost,omitempty"`
	// BackendPort overrides Port for the real backend endpoint mapping.
	// +optional
	BackendPort int32 `json:"backendPort,omitempty"`
}

// Scenario is a named set of virtual responses.
type Scenario struct {
	Name      string                              `json:"name"`
	Responses map[string][]ScenarioOperationBody `json:"responses"`
}

// ScenarioOperationBody is one virtual response for an operation.
type ScenarioOperationBody struct {
	Operation string `json:"operation"`
	Body      string `json:"body"`
}

// Phase values for status.
const (
	PhasePending   = "Pending"
	PhaseReady     = "Ready"
	PhaseError     = "Error"
	PhaseForbidden = "Forbidden"
)

// SimulationManifestStatus is observed state.
type SimulationManifestStatus struct {
	// Phase is Pending | Ready | Error | Forbidden.
	// +optional
	Phase string `json:"phase,omitempty"`
	// Message is a human-readable status detail.
	// +optional
	Message string `json:"message,omitempty"`
	// GeneratedResources lists Kubernetes names created/owned by the operator.
	// +optional
	GeneratedResources []string `json:"generatedResources,omitempty"`
	// ObservedGeneration is the last reconciled metadata.generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=simm
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="ThirdParties",type=integer,JSONPath=`.status.thirdPartyCount`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// SimulationManifest is the Schema for the simulationmanifests API.
type SimulationManifest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SimulationManifestSpec   `json:"spec,omitempty"`
	Status SimulationManifestStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SimulationManifestList contains a list of SimulationManifest.
type SimulationManifestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SimulationManifest `json:"items"`
}
