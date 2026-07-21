package generator_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	simv1 "github.com/servicemesh/virtualization-framework/api/simulation/v1alpha1"
	"github.com/servicemesh/virtualization-framework/internal/config"
	"github.com/servicemesh/virtualization-framework/internal/generator"
)

// updateGoldens regenerates testdata when set:
//
//	UPDATE_GOLDEN=1 go test ./internal/generator/ -run Golden
var updateGoldens = flag.Bool("update-golden", false, "update generator golden files")

func fixedConfig() config.Config {
	return config.Config{
		Environment:             "dev",
		SystemNamespace:         "simulation-system",
		DefaultMicrocksHostPort: "microcks-svc.simulation-system.svc.cluster.local:9090",
		SimulationHeader:        "test-data-simulation-action-name",
		PropagationLabelKey:     "simulation.io/propagation",
		PropagationLabelValue:   "enabled",
	}
}

func goldenManifest() *simv1.SimulationManifest {
	// Stable fixture aligned with examples/reference-app-with-framework.
	return &simv1.SimulationManifest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "reference-app-simulation",
			Namespace: "poc",
		},
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
					"external-risk": {{Operation: "EvaluateRisk", Body: `{"risk_score":92,"decision":"DECLINE"}`}},
				},
			}},
		},
	}
}

func TestGenerate_GoldenReferenceApp(t *testing.T) {
	res, err := generator.Generate(goldenManifest(), fixedConfig())
	if err != nil {
		t.Fatal(err)
	}
	got, err := encodeGolden(res.Objects)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join("testdata", "reference-app.golden.yaml")
	if *updateGoldens || os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("updated %s", path)
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (run UPDATE_GOLDEN=1 go test ./internal/generator/ -run Golden): %v", err)
	}
	if !bytes.Equal(bytes.TrimSpace(want), bytes.TrimSpace(got)) {
		t.Fatalf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s\n\nRegenerate: UPDATE_GOLDEN=1 go test ./internal/generator/ -run Golden",
			path, got, want)
	}
}

func encodeGolden(objs []unstructured.Unstructured) ([]byte, error) {
	// Stable order: Kind, then Name.
	sort.Slice(objs, func(i, j int) bool {
		if objs[i].GetKind() != objs[j].GetKind() {
			return objs[i].GetKind() < objs[j].GetKind()
		}
		return objs[i].GetName() < objs[j].GetName()
	})

	var buf bytes.Buffer
	for i := range objs {
		// Drop volatile / runtime-only fields if any sneak in.
		u := objs[i].DeepCopy()
		unstructured.RemoveNestedField(u.Object, "metadata", "resourceVersion")
		unstructured.RemoveNestedField(u.Object, "metadata", "uid")
		unstructured.RemoveNestedField(u.Object, "metadata", "generation")
		unstructured.RemoveNestedField(u.Object, "metadata", "creationTimestamp")
		unstructured.RemoveNestedField(u.Object, "metadata", "managedFields")

		b, err := yaml.Marshal(u.Object)
		if err != nil {
			return nil, err
		}
		if i > 0 {
			buf.WriteString("---\n")
		}
		buf.Write(b)
		if !bytes.HasSuffix(b, []byte("\n")) {
			buf.WriteByte('\n')
		}
	}
	return buf.Bytes(), nil
}
