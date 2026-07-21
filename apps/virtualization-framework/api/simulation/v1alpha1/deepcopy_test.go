package v1alpha1_test

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	simv1 "github.com/servicemesh/virtualization-framework/api/simulation/v1alpha1"
)

func TestDeepCopy_SimulationManifest(t *testing.T) {
	in := &simv1.SimulationManifest{
		ObjectMeta: metav1.ObjectMeta{Name: "n", Namespace: "ns", Labels: map[string]string{"a": "b"}},
		Spec: simv1.SimulationManifestSpec{
			ThirdParties: []simv1.ThirdParty{{Name: "t", Host: "h", Port: 1}},
			Scenarios: []simv1.Scenario{{
				Name: "s",
				Responses: map[string][]simv1.ScenarioOperationBody{
					"t": {{Operation: "Op", Body: "{}"}},
					"nil": nil,
				},
			}},
			MicrocksService: "m:1",
		},
		Status: simv1.SimulationManifestStatus{
			Phase:              simv1.PhaseReady,
			GeneratedResources: []string{"a", "b"},
		},
	}
	out := in.DeepCopy()
	if out == in || out.Name != "n" {
		t.Fatal("deepcopy failed")
	}
	out.Spec.ThirdParties[0].Name = "changed"
	if in.Spec.ThirdParties[0].Name != "t" {
		t.Fatal("shared slice")
	}
	out.Status.GeneratedResources[0] = "x"
	if in.Status.GeneratedResources[0] != "a" {
		t.Fatal("shared status")
	}
	obj := in.DeepCopyObject()
	if obj == nil {
		t.Fatal("DeepCopyObject")
	}
}

func TestDeepCopy_List(t *testing.T) {
	in := &simv1.SimulationManifestList{
		Items: []simv1.SimulationManifest{
			{ObjectMeta: metav1.ObjectMeta{Name: "a"}},
		},
	}
	out := in.DeepCopy()
	if len(out.Items) != 1 || out.Items[0].Name != "a" {
		t.Fatal(out)
	}
	if in.DeepCopyObject() == nil {
		t.Fatal("list DeepCopyObject")
	}
	// nil receivers
	var nilM *simv1.SimulationManifest
	if nilM.DeepCopy() != nil || nilM.DeepCopyObject() != nil {
		t.Fatal("nil manifest")
	}
	var nilL *simv1.SimulationManifestList
	if nilL.DeepCopy() != nil || nilL.DeepCopyObject() != nil {
		t.Fatal("nil list")
	}
}

func TestDeepCopy_EmptySlices(t *testing.T) {
	in := &simv1.SimulationManifest{
		Spec: simv1.SimulationManifestSpec{},
	}
	out := in.DeepCopy()
	if out == nil {
		t.Fatal("nil")
	}
}
