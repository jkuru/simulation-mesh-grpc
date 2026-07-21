package generator_test

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	simv1 "github.com/servicemesh/virtualization-framework/api/simulation/v1alpha1"
	"github.com/servicemesh/virtualization-framework/internal/config"
	"github.com/servicemesh/virtualization-framework/internal/generator"
)

func sampleManifest() *simv1.SimulationManifest {
	return &simv1.SimulationManifest{
		ObjectMeta: metav1.ObjectMeta{Name: "poc-simulation", Namespace: "poc"},
		Spec: simv1.SimulationManifestSpec{
			ThirdParties: []simv1.ThirdParty{
				{
					Name:        "external_risk",
					Host:        "external-risk-api.com",
					Port:        9003,
					BackendHost: "external-risk.poc.svc.cluster.local",
					BackendPort: 9003,
				},
			},
			Scenarios: []simv1.Scenario{
				{Name: "fraud-declined", Responses: map[string][]simv1.ScenarioOperationBody{
					"external-risk": {{Operation: "EvaluateRisk", Body: `{"decision":"DECLINE"}`}},
				}},
			},
			MicrocksService: "microcks.example:9090",
		},
	}
}

func TestGenerate_ProducesExpectedKinds(t *testing.T) {
	cfg := config.FromEnv()
	res, err := generator.Generate(sampleManifest(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]int{}
	for _, o := range res.Objects {
		kinds[o.GetKind()]++
	}
	if kinds["EnvoyFilter"] != 2 {
		t.Fatalf("EnvoyFilter count = %d", kinds["EnvoyFilter"])
	}
	if kinds["ServiceEntry"] != 1 {
		t.Fatalf("ServiceEntry count = %d", kinds["ServiceEntry"])
	}
	if kinds["VirtualService"] != 1 {
		t.Fatalf("VirtualService count = %d", kinds["VirtualService"])
	}
	if kinds["DestinationRule"] != 2 {
		t.Fatalf("DestinationRule count = %d", kinds["DestinationRule"])
	}
	if len(res.Names) != len(res.Objects) {
		t.Fatalf("names/objects mismatch")
	}
}

func TestGenerate_VirtualServiceRoutes(t *testing.T) {
	res, err := generator.Generate(sampleManifest(), config.FromEnv())
	if err != nil {
		t.Fatal(err)
	}
	var vs map[string]interface{}
	for i := range res.Objects {
		if res.Objects[i].GetKind() == "VirtualService" {
			vs = res.Objects[i].Object
			break
		}
	}
	if vs == nil {
		t.Fatal("no VirtualService")
	}
	http, ok, _ := unstructuredNestedSlice(vs, "spec", "http")
	if !ok || len(http) != 2 {
		t.Fatalf("http routes = %v", http)
	}
}

func TestGenerate_NilManifest(t *testing.T) {
	_, err := generator.Generate(nil, config.FromEnv())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGenerate_EmptyThirdParties(t *testing.T) {
	m := sampleManifest()
	m.Spec.ThirdParties = nil
	_, err := generator.Generate(m, config.FromEnv())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGenerate_EmptyScenarios(t *testing.T) {
	m := sampleManifest()
	m.Spec.Scenarios = nil
	_, err := generator.Generate(m, config.FromEnv())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGenerate_InvalidThirdParty(t *testing.T) {
	m := sampleManifest()
	m.Spec.ThirdParties[0].Name = ""
	_, err := generator.Generate(m, config.FromEnv())
	if err == nil {
		t.Fatal("expected error")
	}
	m = sampleManifest()
	m.Spec.ThirdParties[0].Host = ""
	_, err = generator.Generate(m, config.FromEnv())
	if err == nil {
		t.Fatal("expected host error")
	}
	m = sampleManifest()
	m.Spec.ThirdParties[0].Port = 0
	_, err = generator.Generate(m, config.FromEnv())
	if err == nil {
		t.Fatal("expected port error")
	}
}

func TestGenerate_SplitHostPortEdgeCases(t *testing.T) {
	m := sampleManifest()
	// remainder after last ':' contains ']' so port split is skipped
	m.Spec.MicrocksService = "a:b]"
	_, err := generator.Generate(m, config.FromEnv())
	if err != nil {
		t.Fatal(err)
	}
	m.Spec.MicrocksService = "   "
	_, err = generator.Generate(m, config.FromEnv())
	if err != nil {
		t.Fatal(err)
	}
	// firstNonEmpty prefers non-space MicrocksService already tested; empty scenarios covered
	m = sampleManifest()
	m.Spec.ThirdParties[0].Name = "a/b.c_d"
	_, err = generator.Generate(m, config.FromEnv())
	if err != nil {
		t.Fatal(err)
	}
}

func TestGenerate_DefaultBackendPortAndHost(t *testing.T) {
	m := sampleManifest()
	m.Spec.ThirdParties[0].BackendHost = ""
	m.Spec.ThirdParties[0].BackendPort = 0
	m.Spec.MicrocksService = ""
	cfg := config.FromEnv()
	cfg.DefaultMicrocksHostPort = "" // force splitHostPort empty → default host
	res, err := generator.Generate(m, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Objects) == 0 {
		t.Fatal("empty")
	}
}

func TestGenerate_MicrocksServiceNoPort(t *testing.T) {
	m := sampleManifest()
	m.Spec.MicrocksService = "microcks-svc.simulation-system.svc.cluster.local"
	_, err := generator.Generate(m, config.FromEnv())
	if err != nil {
		t.Fatal(err)
	}
}

func TestGenerate_MicrocksServiceBadPort(t *testing.T) {
	m := sampleManifest()
	m.Spec.MicrocksService = "host:notaport"
	_, err := generator.Generate(m, config.FromEnv())
	if err != nil {
		t.Fatal(err)
	}
}

func unstructuredNestedSlice(obj map[string]interface{}, fields ...string) ([]interface{}, bool, error) {
	cur := interface{}(obj)
	for _, f := range fields {
		m, ok := cur.(map[string]interface{})
		if !ok {
			return nil, false, nil
		}
		cur, ok = m[f]
		if !ok {
			return nil, false, nil
		}
	}
	s, ok := cur.([]interface{})
	return s, ok, nil
}

func TestGenerate_MicrocksBackendAddsRewrite(t *testing.T) {
	m := sampleManifest()
	m.Spec.VirtualBackend = "microcks"
	cfg := config.FromEnv()
	cfg.SystemNamespace = "simulation-system"
	cfg.MicrocksOperationHeader = "x-microcks-operation"
	res, err := generator.Generate(m, cfg)
	if err != nil {
		t.Fatal(err)
	}
	var rewrite *unstructured.Unstructured
	ef := 0
	for i := range res.Objects {
		if res.Objects[i].GetKind() == "EnvoyFilter" {
			ef++
			if res.Objects[i].GetName() == "vf-microcks-scenario-rewrite" {
				o := res.Objects[i]
				rewrite = &o
			}
		}
	}
	if ef != 3 {
		t.Fatalf("EnvoyFilter count = %d want 3", ef)
	}
	if rewrite == nil {
		t.Fatal("missing microcks rewrite filter")
	}
	if rewrite.GetNamespace() != "simulation-system" {
		t.Fatalf("rewrite ns = %s", rewrite.GetNamespace())
	}

	// Empty header config falls back to contract defaults in rewrite filter.
	cfg.SimulationHeader = ""
	cfg.MicrocksOperationHeader = ""
	res2, err := generator.Generate(m, cfg)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := range res2.Objects {
		if res2.Objects[i].GetName() == "vf-microcks-scenario-rewrite" {
			found = true
		}
	}
	if !found {
		t.Fatal("rewrite with default headers missing")
	}
}
