package admission_test

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	simv1 "github.com/servicemesh/virtualization-framework/api/simulation/v1alpha1"
	"github.com/servicemesh/virtualization-framework/internal/admission"
	"github.com/servicemesh/virtualization-framework/internal/config"
)

func TestValidator_CreateUpdateDelete(t *testing.T) {
	v := &admission.Validator{Config: config.Config{Environment: "dev"}}
	m := validManifest()

	if _, err := v.ValidateCreate(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	if _, err := v.ValidateUpdate(context.Background(), m, m); err != nil {
		t.Fatal(err)
	}
	if _, err := v.ValidateDelete(context.Background(), m); err != nil {
		t.Fatal(err)
	}
}

func TestValidator_DenyBad(t *testing.T) {
	v := &admission.Validator{Config: config.Config{Environment: "dev"}}
	m := validManifest()
	m.Spec.ThirdParties = nil
	if _, err := v.ValidateCreate(context.Background(), m); err == nil {
		t.Fatal("expected deny")
	}
	if _, err := v.ValidateUpdate(context.Background(), m, m); err == nil {
		t.Fatal("expected deny")
	}
}

func TestValidator_DenyProd(t *testing.T) {
	v := &admission.Validator{Config: config.Config{Environment: "production"}}
	if _, err := v.ValidateCreate(context.Background(), validManifest()); err == nil {
		t.Fatal("expected prod deny")
	}
}

func TestValidator_WrongType(t *testing.T) {
	v := &admission.Validator{Config: config.Config{}}
	// use a non-manifest runtime.Object
	var obj runtime.Object = &simv1.SimulationManifestList{
		TypeMeta: metav1.TypeMeta{Kind: "SimulationManifestList"},
	}
	if _, err := v.ValidateCreate(context.Background(), obj); err == nil {
		t.Fatal("expected type error")
	}
	if _, err := v.ValidateUpdate(context.Background(), obj, obj); err == nil {
		t.Fatal("expected type error")
	}
}
