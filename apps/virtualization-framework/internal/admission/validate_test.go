package admission_test

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	simv1 "github.com/servicemesh/virtualization-framework/api/simulation/v1alpha1"
	"github.com/servicemesh/virtualization-framework/internal/admission"
	"github.com/servicemesh/virtualization-framework/internal/config"
)

func validManifest() *simv1.SimulationManifest {
	return &simv1.SimulationManifest{
		ObjectMeta: metav1.ObjectMeta{Name: "ref", Namespace: "poc"},
		Spec: simv1.SimulationManifestSpec{
			MicrocksService: "microcks-svc.simulation-system.svc.cluster.local:9090",
			ThirdParties: []simv1.ThirdParty{{
				Name:        "external-risk",
				Host:        "external-risk-api.com",
				Port:        9003,
				BackendHost: "external-risk.poc.svc.cluster.local",
				BackendPort: 9003,
			}},
			Scenarios: []simv1.Scenario{{
				Name: "fraud-declined",
				Responses: map[string][]simv1.ScenarioOperationBody{
					"external-risk": {{Operation: "EvaluateRisk", Body: `{"risk_score":92}`}},
				},
			}},
		},
	}
}

func TestValidateSpec_OK(t *testing.T) {
	if errs := admission.ValidateSpec(validManifest()); len(errs) != 0 {
		t.Fatalf("unexpected: %v", errs)
	}
}

func TestValidateSpec_Nil(t *testing.T) {
	if errs := admission.ValidateSpec(nil); len(errs) == 0 {
		t.Fatal("expected error")
	}
}

func TestValidateSpec_MissingCollections(t *testing.T) {
	m := validManifest()
	m.Spec.ThirdParties = nil
	m.Spec.Scenarios = nil
	errs := admission.ValidateSpec(m)
	if len(errs) < 2 {
		t.Fatalf("expected both required errors, got %v", errs)
	}
}

func TestValidateSpec_ThirdPartyFields(t *testing.T) {
	m := validManifest()
	m.Spec.ThirdParties = []simv1.ThirdParty{{
		Name: "Bad_Name", Host: "http://x.com/path", Port: 0, BackendPort: 99999, BackendHost: "https://bad",
	}}
	errs := admission.ValidateSpec(m)
	if len(errs) < 4 {
		t.Fatalf("expected multiple field errors, got %v", errs)
	}
}

func TestValidateSpec_DuplicateNameAndHost(t *testing.T) {
	m := validManifest()
	m.Spec.ThirdParties = []simv1.ThirdParty{
		{Name: "a", Host: "h.com", Port: 1},
		{Name: "a", Host: "h.com", Port: 2},
	}
	// also fix scenarios to reference a
	m.Spec.Scenarios[0].Responses = map[string][]simv1.ScenarioOperationBody{
		"a": {{Operation: "Op", Body: "{}"}},
	}
	errs := admission.ValidateSpec(m)
	if len(errs) < 2 {
		t.Fatalf("expected duplicates, got %v", errs)
	}
}

func TestValidateSpec_ScenarioErrors(t *testing.T) {
	m := validManifest()
	m.Spec.Scenarios = []simv1.Scenario{
		{Name: "", Responses: nil},
		{Name: "BadName", Responses: map[string][]simv1.ScenarioOperationBody{
			"missing-tp": {},
		}},
		{Name: "ok-name", Responses: map[string][]simv1.ScenarioOperationBody{
			"external-risk": {{Operation: "", Body: ""}},
		}},
	}
	// duplicate name
	m.Spec.Scenarios = append(m.Spec.Scenarios, simv1.Scenario{
		Name: "ok-name",
		Responses: map[string][]simv1.ScenarioOperationBody{
			"external-risk": {{Operation: "X", Body: "{}"}},
		},
	})
	errs := admission.ValidateSpec(m)
	if len(errs) < 4 {
		t.Fatalf("expected scenario errors, got %v", errs)
	}
}

func TestValidateSpec_MicrocksService(t *testing.T) {
	m := validManifest()
	m.Spec.MicrocksService = "not-a-host-port"
	if errs := admission.ValidateSpec(m); len(errs) == 0 {
		t.Fatal("expected microcksService error")
	}
	m.Spec.MicrocksService = ":9090"
	if errs := admission.ValidateSpec(m); len(errs) == 0 {
		t.Fatal("expected empty host error")
	}
	m.Spec.MicrocksService = ""
	if errs := admission.ValidateSpec(m); len(errs) != 0 {
		t.Fatalf("empty microcks is optional: %v", errs)
	}
}

func TestValidateForAdmission_Prod(t *testing.T) {
	err := admission.ValidateForAdmission(validManifest(), config.Config{Environment: "prod"})
	if err == nil || !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("got %v", err)
	}
}

func TestValidateForAdmission_ProdLabel(t *testing.T) {
	m := validManifest()
	m.Labels = map[string]string{"simulation.io/environment": "production"}
	if err := admission.ValidateForAdmission(m, config.Config{Environment: "dev"}); err == nil {
		t.Fatal("expected label forbid")
	}
	m.Labels = nil
	m.Annotations = map[string]string{"simulation.io/environment": "prod"}
	if err := admission.ValidateForAdmission(m, config.Config{Environment: "dev"}); err == nil {
		t.Fatal("expected annotation forbid")
	}
}

func TestValidateForAdmission_OK(t *testing.T) {
	if err := admission.ValidateForAdmission(validManifest(), config.Config{Environment: "dev"}); err != nil {
		t.Fatal(err)
	}
}

func TestValidateForAdmission_SpecErrors(t *testing.T) {
	m := validManifest()
	m.Spec.ThirdParties = nil
	if err := admission.ValidateForAdmission(m, config.Config{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestErrorString(t *testing.T) {
	if admission.ErrorString(nil) != "" {
		t.Fatal("nil")
	}
	if admission.ErrorString(admission.ValidateSpec(nil).ToAggregate()) == "" {
		t.Fatal("expected message")
	}
}

func TestValidateSpec_EmptyNameHost(t *testing.T) {
	m := validManifest()
	m.Spec.ThirdParties = []simv1.ThirdParty{{Name: "", Host: "", Port: 1}}
	m.Spec.Scenarios[0].Responses = map[string][]simv1.ScenarioOperationBody{
		"x": {{Operation: "O", Body: "{}"}},
	}
	// response key won't match empty name
	if errs := admission.ValidateSpec(m); len(errs) < 2 {
		t.Fatalf("got %v", errs)
	}
}

func TestValidateSpec_HostWithScheme(t *testing.T) {
	m := validManifest()
	m.Spec.ThirdParties[0].Host = "example.com/path"
	if errs := admission.ValidateSpec(m); len(errs) == 0 {
		t.Fatal("expected path invalid")
	}
}

func TestValidateSpec_HostDNSInvalid(t *testing.T) {
	m := validManifest()
	m.Spec.ThirdParties[0].Host = "bad_host"
	if errs := admission.ValidateSpec(m); len(errs) == 0 {
		t.Fatal("expected host DNS invalid")
	}
	m = validManifest()
	m.Spec.ThirdParties[0].BackendHost = "also_bad!"
	if errs := admission.ValidateSpec(m); len(errs) == 0 {
		t.Fatal("expected backendHost DNS invalid")
	}
}

func TestValidateSpec_MicrocksEmptyPort(t *testing.T) {
	m := validManifest()
	m.Spec.MicrocksService = "microcks.svc:"
	if errs := admission.ValidateSpec(m); len(errs) == 0 {
		t.Fatal("expected empty port error")
	}
}

func TestValidateSpec_BadVirtualBackend(t *testing.T) {
	m := validManifest()
	m.Spec.VirtualBackend = "not-a-backend"
	if errs := admission.ValidateSpec(m); len(errs) == 0 {
		t.Fatal("expected virtualBackend error")
	}
}
