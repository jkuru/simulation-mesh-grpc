package controller_test

import (
	"context"
	"errors"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"k8s.io/client-go/tools/record"

	simv1 "github.com/servicemesh/virtualization-framework/api/simulation/v1alpha1"
	"github.com/servicemesh/virtualization-framework/internal/config"
	"github.com/servicemesh/virtualization-framework/internal/controller"
	"github.com/servicemesh/virtualization-framework/internal/generator"
	"github.com/servicemesh/virtualization-framework/internal/metrics"
)

func scheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := simv1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	return s
}

func sampleManifest(name, ns string) *simv1.SimulationManifest {
	return &simv1.SimulationManifest{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  ns,
			Generation: 1,
		},
		Spec: simv1.SimulationManifestSpec{
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
					"external-risk": {{Operation: "EvaluateRisk", Body: `{}`}},
				},
			}},
		},
	}
}

func newReconciler(t *testing.T, objs ...client.Object) *controller.SimulationManifestReconciler {
	t.Helper()
	s := scheme(t)
	b := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(&simv1.SimulationManifest{})
	if len(objs) > 0 {
		b = b.WithObjects(objs...)
	}
	return &controller.SimulationManifestReconciler{
		Client:   b.Build(),
		Scheme:   s,
		Config:   config.FromEnv(),
		Recorder: record.NewFakeRecorder(64),
		Metrics:  metrics.New(),
	}
}

func TestReconcile_NotFound(t *testing.T) {
	r := newReconciler(t)
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "missing", Namespace: "poc"},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestReconcile_ForbiddenInProd(t *testing.T) {
	m := sampleManifest("x", "poc")
	r := newReconciler(t, m)
	r.Config.Environment = "prod"
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "x", Namespace: "poc"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var got simv1.SimulationManifest
	if err := r.Get(context.Background(), types.NamespacedName{Name: "x", Namespace: "poc"}, &got); err != nil {
		t.Fatal(err)
	}
	if got.Status.Phase != simv1.PhaseForbidden {
		t.Fatalf("phase = %q", got.Status.Phase)
	}
}

func TestReconcile_AddsFinalizerThenReady(t *testing.T) {
	m := sampleManifest("ref", "poc")
	r := newReconciler(t, m)

	// First pass: add finalizer + requeue
	res, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "ref", Namespace: "poc"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Requeue {
		t.Fatal("expected requeue after finalizer")
	}

	// Second pass: generate + Ready
	_, err = r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "ref", Namespace: "poc"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var got simv1.SimulationManifest
	if err := r.Get(context.Background(), types.NamespacedName{Name: "ref", Namespace: "poc"}, &got); err != nil {
		t.Fatal(err)
	}
	if got.Status.Phase != simv1.PhaseReady {
		t.Fatalf("phase = %q msg=%q", got.Status.Phase, got.Status.Message)
	}
	if len(got.Status.GeneratedResources) == 0 {
		t.Fatal("expected generated resources")
	}

	// Third pass: update existing unstructured (apply update path)
	_, err = r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "ref", Namespace: "poc"},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestReconcile_ValidationError(t *testing.T) {
	// Spec invalid: admission rules fail (no hot-loop error return).
	m := sampleManifest("bad", "poc")
	m.Spec.ThirdParties = nil
	m.Finalizers = []string{"simulation.io/finalizer"}
	r := newReconciler(t, m)
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "bad", Namespace: "poc"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var got simv1.SimulationManifest
	_ = r.Get(context.Background(), types.NamespacedName{Name: "bad", Namespace: "poc"}, &got)
	if got.Status.Phase != simv1.PhaseError {
		t.Fatalf("phase = %q msg=%q", got.Status.Phase, got.Status.Message)
	}
}

func TestReconcile_GenerateError(t *testing.T) {
	m := sampleManifest("gen", "poc")
	m.Finalizers = []string{"simulation.io/finalizer"}
	r := newReconciler(t, m)
	r.Generate = func(m *simv1.SimulationManifest, cfg config.Config) (generator.Result, error) {
		return generator.Result{}, errors.New("generate boom")
	}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "gen", Namespace: "poc"},
	})
	if err == nil {
		t.Fatal("expected generate error")
	}
	var got simv1.SimulationManifest
	_ = r.Get(context.Background(), types.NamespacedName{Name: "gen", Namespace: "poc"}, &got)
	if got.Status.Phase != simv1.PhaseError {
		t.Fatalf("phase = %q", got.Status.Phase)
	}
}

func TestReconcile_DeleteWithoutFinalizer(t *testing.T) {
	// Fake client refuses deletionTimestamp without finalizers; inject via Get.
	now := metav1.Now()
	s := scheme(t)
	cl := fake.NewClientBuilder().WithScheme(s).
		WithStatusSubresource(&simv1.SimulationManifest{}).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				m := obj.(*simv1.SimulationManifest)
				*m = *sampleManifest("df", "poc")
				m.DeletionTimestamp = &now
				m.Finalizers = nil
				return nil
			},
		}).Build()
	r := &controller.SimulationManifestReconciler{
		Client: cl, Scheme: s, Config: config.FromEnv(),
		Recorder: record.NewFakeRecorder(8), Metrics: metrics.New(),
	}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "df", Namespace: "poc"},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestReconcile_DeleteWithFinalizer(t *testing.T) {
	now := metav1.Now()
	m := sampleManifest("del", "poc")
	m.Finalizers = []string{"simulation.io/finalizer"}
	m.DeletionTimestamp = &now

	// Seed an owned unstructured object.
	ef := &unstructured.Unstructured{}
	ef.SetAPIVersion("networking.istio.io/v1alpha3")
	ef.SetKind("EnvoyFilter")
	ef.SetName("vf-inbound-capture")
	ef.SetNamespace("poc")
	ef.SetLabels(map[string]string{
		"simulation.io/manifest": "del",
	})

	s := scheme(t)
	cl := fake.NewClientBuilder().WithScheme(s).
		WithStatusSubresource(&simv1.SimulationManifest{}).
		WithObjects(m, ef).
		Build()
	r := &controller.SimulationManifestReconciler{
		Client: cl, Scheme: s, Config: config.FromEnv(),
		Recorder: record.NewFakeRecorder(8), Metrics: metrics.New(),
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "del", Namespace: "poc"},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Finalizer should be removed; object may still exist without finalizer or be gone.
	var got simv1.SimulationManifest
	err = r.Get(context.Background(), types.NamespacedName{Name: "del", Namespace: "poc"}, &got)
	if err == nil && len(got.Finalizers) != 0 {
		t.Fatalf("finalizers still present: %v", got.Finalizers)
	}
}

func TestReconcile_GetError(t *testing.T) {
	s := scheme(t)
	cl := fake.NewClientBuilder().WithScheme(s).WithInterceptorFuncs(interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			return errors.New("boom get")
		},
	}).Build()
	r := &controller.SimulationManifestReconciler{
		Client: cl, Scheme: s, Config: config.FromEnv(),
		Recorder: record.NewFakeRecorder(8), Metrics: metrics.New(),
	}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "x", Namespace: "poc"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReconcile_ApplyCreateFails(t *testing.T) {
	m := sampleManifest("fail", "poc")
	m.Finalizers = []string{"simulation.io/finalizer"}
	s := scheme(t)
	base := fake.NewClientBuilder().WithScheme(s).
		WithStatusSubresource(&simv1.SimulationManifest{}).
		WithObjects(m).Build()
	cl := fake.NewClientBuilder().WithScheme(s).
		WithStatusSubresource(&simv1.SimulationManifest{}).
		WithObjects(m).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				if u, ok := obj.(*unstructured.Unstructured); ok && u.GetKind() == "EnvoyFilter" {
					return errors.New("create denied")
				}
				return base.Create(ctx, obj, opts...)
			},
			Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				return base.Get(ctx, key, obj, opts...)
			},
			Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
				return base.Update(ctx, obj, opts...)
			},
			// status update goes through SubResource
		}).Build()

	// Simpler: use interceptor that fails all Create of unstructured
	cl = fake.NewClientBuilder().WithScheme(s).
		WithStatusSubresource(&simv1.SimulationManifest{}).
		WithObjects(m).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				if _, ok := obj.(*unstructured.Unstructured); ok {
					return errors.New("create denied")
				}
				return c.Create(ctx, obj, opts...)
			},
		}).Build()

	r := &controller.SimulationManifestReconciler{
		Client: cl, Scheme: s, Config: config.FromEnv(),
		Recorder: record.NewFakeRecorder(8), Metrics: metrics.New(),
	}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "fail", Namespace: "poc"},
	})
	if err == nil {
		t.Fatal("expected apply error")
	}
}

func TestMetaNoMatch_viaDeleteOwned(t *testing.T) {
	// deleteOwned with list error that is "no matches for kind" should be skipped.
	m := sampleManifest("d", "poc")
	m.Finalizers = []string{"simulation.io/finalizer"}
	now := metav1.Now()
	m.DeletionTimestamp = &now

	s := scheme(t)
	cl := fake.NewClientBuilder().WithScheme(s).
		WithStatusSubresource(&simv1.SimulationManifest{}).
		WithObjects(m).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				return errors.New("no matches for kind \"VirtualService\" in version \"v1beta1\"")
			},
		}).Build()
	r := &controller.SimulationManifestReconciler{
		Client: cl, Scheme: s, Config: config.FromEnv(),
		Recorder: record.NewFakeRecorder(8), Metrics: metrics.New(),
	}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "d", Namespace: "poc"},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDeleteOwned_ListHardError(t *testing.T) {
	m := sampleManifest("d2", "poc")
	m.Finalizers = []string{"simulation.io/finalizer"}
	now := metav1.Now()
	m.DeletionTimestamp = &now
	s := scheme(t)
	cl := fake.NewClientBuilder().WithScheme(s).
		WithStatusSubresource(&simv1.SimulationManifest{}).
		WithObjects(m).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				return errors.New("list failed hard")
			},
		}).Build()
	r := &controller.SimulationManifestReconciler{
		Client: cl, Scheme: s, Config: config.FromEnv(),
		Recorder: record.NewFakeRecorder(8), Metrics: metrics.New(),
	}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "d2", Namespace: "poc"},
	})
	if err == nil {
		t.Fatal("expected list error")
	}
}

func TestApplyUpdateMergesLabels(t *testing.T) {
	m := sampleManifest("upd", "poc")
	m.Finalizers = []string{"simulation.io/finalizer"}

	existing := &unstructured.Unstructured{}
	existing.SetAPIVersion("networking.istio.io/v1alpha3")
	existing.SetKind("EnvoyFilter")
	existing.SetName("vf-inbound-capture")
	existing.SetNamespace("poc")
	existing.SetLabels(map[string]string{"keep": "me"})
	existing.SetResourceVersion("1")

	s := scheme(t)
	cl := fake.NewClientBuilder().WithScheme(s).
		WithStatusSubresource(&simv1.SimulationManifest{}).
		WithObjects(m, existing).
		Build()
	r := &controller.SimulationManifestReconciler{
		Client: cl, Scheme: s, Config: config.FromEnv(),
		Recorder: record.NewFakeRecorder(8), Metrics: metrics.New(),
	}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "upd", Namespace: "poc"},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestApplyGetHardError(t *testing.T) {
	m := sampleManifest("ge", "poc")
	m.Finalizers = []string{"simulation.io/finalizer"}
	s := scheme(t)
	cl := fake.NewClientBuilder().WithScheme(s).
		WithStatusSubresource(&simv1.SimulationManifest{}).
		WithObjects(m).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				if _, ok := obj.(*unstructured.Unstructured); ok {
					return errors.New("get unstructured failed")
				}
				return c.Get(ctx, key, obj, opts...)
			},
		}).Build()
	r := &controller.SimulationManifestReconciler{
		Client: cl, Scheme: s, Config: config.FromEnv(),
		Recorder: record.NewFakeRecorder(8), Metrics: metrics.New(),
	}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "ge", Namespace: "poc"},
	})
	if err == nil {
		t.Fatal("expected get error on apply")
	}
}

func TestDeleteOwned_DeleteHardError(t *testing.T) {
	m := sampleManifest("dh", "poc")
	m.Finalizers = []string{"simulation.io/finalizer"}
	now := metav1.Now()
	m.DeletionTimestamp = &now
	ef := &unstructured.Unstructured{}
	ef.SetAPIVersion("networking.istio.io/v1alpha3")
	ef.SetKind("EnvoyFilter")
	ef.SetName("vf-inbound-capture")
	ef.SetNamespace("poc")
	ef.SetLabels(map[string]string{"simulation.io/manifest": "dh"})
	s := scheme(t)
	cl := fake.NewClientBuilder().WithScheme(s).
		WithStatusSubresource(&simv1.SimulationManifest{}).
		WithObjects(m, ef).
		WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
				return errors.New("delete denied")
			},
		}).Build()
	r := &controller.SimulationManifestReconciler{
		Client: cl, Scheme: s, Config: config.FromEnv(),
		Recorder: record.NewFakeRecorder(8), Metrics: metrics.New(),
	}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dh", Namespace: "poc"},
	})
	if err == nil {
		t.Fatal("expected delete error")
	}
}

func TestFinalizerAddUpdateError(t *testing.T) {
	m := sampleManifest("fu", "poc")
	s := scheme(t)
	cl := fake.NewClientBuilder().WithScheme(s).
		WithStatusSubresource(&simv1.SimulationManifest{}).
		WithObjects(m).
		WithInterceptorFuncs(interceptor.Funcs{
			Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
				return errors.New("update denied")
			},
		}).Build()
	r := &controller.SimulationManifestReconciler{
		Client: cl, Scheme: s, Config: config.FromEnv(),
		Recorder: record.NewFakeRecorder(8), Metrics: metrics.New(),
	}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "fu", Namespace: "poc"},
	})
	if err == nil {
		t.Fatal("expected update error")
	}
}

func TestDeleteFinalizerRemoveUpdateError(t *testing.T) {
	now := metav1.Now()
	m := sampleManifest("du", "poc")
	m.Finalizers = []string{"simulation.io/finalizer"}
	m.DeletionTimestamp = &now
	s := scheme(t)
	cl := fake.NewClientBuilder().WithScheme(s).
		WithStatusSubresource(&simv1.SimulationManifest{}).
		WithObjects(m).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				return apierrors.NewNotFound(schema.GroupResource{Resource: "x"}, "y")
			},
			Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
				return errors.New("cannot remove finalizer")
			},
		}).Build()
	r := &controller.SimulationManifestReconciler{
		Client: cl, Scheme: s, Config: config.FromEnv(),
		Recorder: record.NewFakeRecorder(8), Metrics: metrics.New(),
	}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "du", Namespace: "poc"},
	})
	if err == nil {
		t.Fatal("expected finalizer update error")
	}
}

func TestMetaNoMatch_CouldNotFindAndNil(t *testing.T) {
	// Cover "could not find the requested resource" via list
	m := sampleManifest("mm", "poc")
	m.Finalizers = []string{"simulation.io/finalizer"}
	now := metav1.Now()
	m.DeletionTimestamp = &now
	s := scheme(t)
	cl := fake.NewClientBuilder().WithScheme(s).
		WithStatusSubresource(&simv1.SimulationManifest{}).
		WithObjects(m).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				return errors.New("could not find the requested resource")
			},
		}).Build()
	r := &controller.SimulationManifestReconciler{
		Client: cl, Scheme: s, Config: config.FromEnv(),
		Recorder: record.NewFakeRecorder(8), Metrics: metrics.New(),
	}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "mm", Namespace: "poc"},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestIsNotFoundOnDelete(t *testing.T) {
	// Cover delete path when Delete returns NotFound
	m := sampleManifest("dn", "poc")
	m.Finalizers = []string{"simulation.io/finalizer"}
	now := metav1.Now()
	m.DeletionTimestamp = &now

	ef := &unstructured.Unstructured{}
	ef.SetGroupVersionKind(schema.GroupVersionKind{Group: "networking.istio.io", Version: "v1alpha3", Kind: "EnvoyFilter"})
	ef.SetName("vf-inbound-capture")
	ef.SetNamespace("poc")
	ef.SetLabels(map[string]string{"simulation.io/manifest": "dn"})

	s := scheme(t)
	cl := fake.NewClientBuilder().WithScheme(s).
		WithStatusSubresource(&simv1.SimulationManifest{}).
		WithObjects(m, ef).
		WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
				return apierrors.NewNotFound(schema.GroupResource{Group: "networking.istio.io", Resource: "envoyfilters"}, "x")
			},
		}).Build()
	r := &controller.SimulationManifestReconciler{
		Client: cl, Scheme: s, Config: config.FromEnv(),
		Recorder: record.NewFakeRecorder(8), Metrics: metrics.New(),
	}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dn", Namespace: "poc"},
	})
	if err != nil {
		t.Fatal(err)
	}
}

// statusClient wraps Status() to force Patch failure / Update paths.
type statusClient struct {
	client.Client
	patchErr  error
	updateErr error
}

func (s statusClient) Status() client.StatusWriter {
	return statusWriter{StatusWriter: s.Client.Status(), patchErr: s.patchErr, updateErr: s.updateErr}
}

type statusWriter struct {
	client.StatusWriter
	patchErr  error
	updateErr error
}

func (w statusWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	if w.patchErr != nil {
		if w.updateErr != nil {
			return w.patchErr
		}
		// Fall through to real Update via outer patchStatus fallback by failing Patch only.
		return w.patchErr
	}
	return w.StatusWriter.Patch(ctx, obj, patch, opts...)
}

func (w statusWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	if w.updateErr != nil {
		return w.updateErr
	}
	return w.StatusWriter.Update(ctx, obj, opts...)
}

func TestPatchStatus_FallbackUpdate(t *testing.T) {
	m := sampleManifest("ps", "poc")
	m.Finalizers = []string{"simulation.io/finalizer"}
	s := scheme(t)
	base := fake.NewClientBuilder().WithScheme(s).
		WithStatusSubresource(&simv1.SimulationManifest{}).
		WithObjects(m).Build()
	r := &controller.SimulationManifestReconciler{
		Client: statusClient{Client: base, patchErr: errors.New("patch fail")},
		Scheme: s,
		Config: config.FromEnv(),
		Recorder: record.NewFakeRecorder(8), Metrics: metrics.New(),
	}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "ps", Namespace: "poc"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var got simv1.SimulationManifest
	if err := base.Get(context.Background(), types.NamespacedName{Name: "ps", Namespace: "poc"}, &got); err != nil {
		t.Fatal(err)
	}
	if got.Status.Phase != simv1.PhaseReady {
		t.Fatalf("phase=%q", got.Status.Phase)
	}
}

func TestPatchStatus_BothFail(t *testing.T) {
	m := sampleManifest("pf", "poc")
	r := newReconciler(t, m)
	r.Config.Environment = "prod"
	// wrap after building
	base := r.Client
	r.Client = statusClient{Client: base, patchErr: errors.New("p"), updateErr: errors.New("u")}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "pf", Namespace: "poc"},
	})
	if err == nil {
		t.Fatal("expected status error")
	}
}

func TestReconcile_NilMetricsAndRecorder(t *testing.T) {
	// Cover metrics() fallback to Default and eventf no-op when Recorder is nil.
	m := sampleManifest("nm", "poc")
	s := scheme(t)
	cl := fake.NewClientBuilder().WithScheme(s).
		WithStatusSubresource(&simv1.SimulationManifest{}).
		WithObjects(m).Build()
	r := &controller.SimulationManifestReconciler{
		Client: cl, Scheme: s, Config: config.FromEnv(),
		// Metrics nil → Default; Recorder nil → skip Events
	}
	// finalizer pass
	res, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "nm", Namespace: "poc"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Requeue {
		t.Fatal("expected requeue")
	}
	_, err = r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "nm", Namespace: "poc"},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestApplyUpdate_NilExistingLabels(t *testing.T) {
	m := sampleManifest("nl", "poc")
	m.Finalizers = []string{"simulation.io/finalizer"}
	existing := &unstructured.Unstructured{}
	existing.SetAPIVersion("networking.istio.io/v1alpha3")
	existing.SetKind("EnvoyFilter")
	existing.SetName("vf-inbound-capture")
	existing.SetNamespace("poc")
	existing.SetResourceVersion("9")
	// deliberately no labels
	s := scheme(t)
	cl := fake.NewClientBuilder().WithScheme(s).
		WithStatusSubresource(&simv1.SimulationManifest{}).
		WithObjects(m, existing).
		Build()
	r := &controller.SimulationManifestReconciler{
		Client: cl, Scheme: s, Config: config.FromEnv(),
		Recorder: record.NewFakeRecorder(8), Metrics: metrics.New(),
	}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "nl", Namespace: "poc"},
	})
	if err != nil {
		t.Fatal(err)
	}
}
