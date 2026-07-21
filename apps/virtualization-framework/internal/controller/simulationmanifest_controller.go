package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	simv1 "github.com/servicemesh/virtualization-framework/api/simulation/v1alpha1"
	"github.com/servicemesh/virtualization-framework/internal/admission"
	"github.com/servicemesh/virtualization-framework/internal/config"
	"github.com/servicemesh/virtualization-framework/internal/events"
	"github.com/servicemesh/virtualization-framework/internal/generator"
	"github.com/servicemesh/virtualization-framework/internal/metrics"
)

const finalizer = "simulation.io/finalizer"

// SimulationManifestReconciler reconciles SimulationManifest objects.
type SimulationManifestReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Config   config.Config
	Recorder record.EventRecorder
	Metrics  *metrics.Collector
	// Generate overrides generator.Generate (tests only).
	Generate func(m *simv1.SimulationManifest, cfg config.Config) (generator.Result, error)
}

// +kubebuilder:rbac:groups=simulation.io,resources=simulationmanifests,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=simulation.io,resources=simulationmanifests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=simulation.io,resources=simulationmanifests/finalizers,verbs=update
// +kubebuilder:rbac:groups=networking.istio.io,resources=virtualservices;serviceentries;destinationrules;envoyfilters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *SimulationManifestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	start := time.Now()
	resultLabel := metrics.ResultSuccess
	defer func() {
		r.metrics().ObserveReconcile(resultLabel, time.Since(start).Seconds())
	}()

	var m simv1.SimulationManifest
	if err := r.Get(ctx, req.NamespacedName, &m); err != nil {
		if apierrors.IsNotFound(err) {
			resultLabel = metrics.ResultDeleted
			return ctrl.Result{}, nil
		}
		resultLabel = metrics.ResultError
		return ctrl.Result{}, err
	}

	// Production safety: never generate virtual routes.
	if r.Config.IsProd() {
		resultLabel = metrics.ResultForbidden
		r.eventf(&m, corev1.EventTypeWarning, events.ReasonForbidden,
			"SimulationManifest is forbidden in production environment")
		return r.patchStatus(ctx, &m, simv1.PhaseForbidden,
			"SimulationManifest is forbidden in production environment", nil)
	}

	if !m.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&m, finalizer) {
			r.eventf(&m, corev1.EventTypeNormal, events.ReasonDeleting, "removing generated Istio resources")
			if err := r.deleteOwned(ctx, &m); err != nil {
				resultLabel = metrics.ResultError
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(&m, finalizer)
			if err := r.Update(ctx, &m); err != nil {
				resultLabel = metrics.ResultError
				return ctrl.Result{}, err
			}
			r.metrics().SetGenerated(m.Namespace, m.Name, 0)
		}
		resultLabel = metrics.ResultDeleted
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(&m, finalizer) {
		controllerutil.AddFinalizer(&m, finalizer)
		if err := r.Update(ctx, &m); err != nil {
			resultLabel = metrics.ResultError
			return ctrl.Result{}, err
		}
		r.eventf(&m, corev1.EventTypeNormal, events.ReasonFinalizerAdded, "added finalizer %s", finalizer)
		resultLabel = metrics.ResultRequeue
		return ctrl.Result{Requeue: true}, nil
	}

	// Defense in depth: same rules as the validating webhook.
	// Permanent spec errors: surface status/Event and do not hot-loop requeue.
	if err := admission.ValidateForAdmission(&m, r.Config); err != nil {
		resultLabel = metrics.ResultError
		logger.Error(err, "validation failed")
		r.eventf(&m, corev1.EventTypeWarning, events.ReasonValidationError, "%s", err.Error())
		return r.patchStatus(ctx, &m, simv1.PhaseError, err.Error(), nil)
	}

	res, err := r.generate(&m)
	if err != nil {
		resultLabel = metrics.ResultError
		logger.Error(err, "generate failed")
		r.eventf(&m, corev1.EventTypeWarning, events.ReasonReconcileError, "generate: %v", err)
		_, _ = r.patchStatus(ctx, &m, simv1.PhaseError, err.Error(), nil)
		return ctrl.Result{}, err
	}

	for i := range res.Objects {
		obj := &res.Objects[i]
		// Owner refs on Istio unstructured types are optional; labels track ownership.
		_ = controllerutil.SetControllerReference(&m, obj, r.Scheme)
		if err := r.applyUnstructured(ctx, obj); err != nil {
			resultLabel = metrics.ResultError
			logger.Error(err, "apply failed", "kind", obj.GetKind(), "name", obj.GetName())
			msg := fmt.Sprintf("apply %s/%s: %v", obj.GetKind(), obj.GetName(), err)
			r.eventf(&m, corev1.EventTypeWarning, events.ReasonReconcileError, "%s", msg)
			_, _ = r.patchStatus(ctx, &m, simv1.PhaseError, msg, res.Names)
			return ctrl.Result{}, err
		}
	}

	r.metrics().SetGenerated(m.Namespace, m.Name, float64(len(res.Names)))
	msg := fmt.Sprintf("generated %d resources", len(res.Names))
	r.eventf(&m, corev1.EventTypeNormal, events.ReasonReady, "%s", msg)
	return r.patchStatus(ctx, &m, simv1.PhaseReady, msg, res.Names)
}

// Note: controller registration lives in cmd/operator (composition root).

func (r *SimulationManifestReconciler) applyUnstructured(ctx context.Context, desired *unstructured.Unstructured) error {
	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(desired.GroupVersionKind())
	key := types.NamespacedName{Name: desired.GetName(), Namespace: desired.GetNamespace()}
	err := r.Get(ctx, key, existing)
	if apierrors.IsNotFound(err) {
		return r.Create(ctx, desired)
	}
	if err != nil {
		return err
	}
	// Preserve resourceVersion for update.
	desired.SetResourceVersion(existing.GetResourceVersion())
	// Merge managed labels.
	labels := existing.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	for k, v := range desired.GetLabels() {
		labels[k] = v
	}
	desired.SetLabels(labels)
	return r.Update(ctx, desired)
}

func (r *SimulationManifestReconciler) deleteOwned(ctx context.Context, m *simv1.SimulationManifest) error {
	// Best-effort delete of labeled children in the app namespace and system ns
	// (microcks rewrite EnvoyFilter is created in SYSTEM_NAMESPACE).
	namespaces := []string{m.Namespace}
	if sys := r.Config.SystemNamespace; sys != "" && sys != m.Namespace {
		namespaces = append(namespaces, sys)
	}
	gvks := []schema.GroupVersionKind{
		{Group: "networking.istio.io", Version: "v1beta1", Kind: "VirtualService"},
		{Group: "networking.istio.io", Version: "v1beta1", Kind: "ServiceEntry"},
		{Group: "networking.istio.io", Version: "v1beta1", Kind: "DestinationRule"},
		{Group: "networking.istio.io", Version: "v1alpha3", Kind: "EnvoyFilter"},
	}
	for _, ns := range namespaces {
		for _, gvk := range gvks {
			list := &unstructured.UnstructuredList{}
			list.SetGroupVersionKind(schema.GroupVersionKind{Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind + "List"})
			if err := r.List(ctx, list, client.InNamespace(ns), client.MatchingLabels{
				"simulation.io/manifest": m.Name,
			}); err != nil {
				// Kind may not be registered on API server (no Istio) — skip.
				if metaNoMatch(err) {
					continue
				}
				return err
			}
			for i := range list.Items {
				item := &list.Items[i]
				if err := r.Delete(ctx, item); err != nil && !apierrors.IsNotFound(err) {
					return err
				}
			}
		}
	}
	return nil
}

func metaNoMatch(err error) bool {
	// Caller only invokes when err != nil.
	s := err.Error()
	return strings.Contains(s, "no matches for kind") ||
		strings.Contains(s, "could not find the requested resource") ||
		apierrors.IsNotFound(err)
}

func (r *SimulationManifestReconciler) patchStatus(ctx context.Context, m *simv1.SimulationManifest, phase, message string, resources []string) (ctrl.Result, error) {
	base := m.DeepCopy()
	m.Status.Phase = phase
	m.Status.Message = message
	m.Status.GeneratedResources = resources
	m.Status.ObservedGeneration = m.Generation
	if err := r.Status().Patch(ctx, m, client.MergeFrom(base)); err != nil {
		// Fallback to Update for environments without patch on status.
		if err2 := r.Status().Update(ctx, m); err2 != nil {
			return ctrl.Result{}, err2
		}
	}
	r.metrics().ObservePhase(phase)
	return ctrl.Result{}, nil
}

func (r *SimulationManifestReconciler) generate(m *simv1.SimulationManifest) (generator.Result, error) {
	if r.Generate != nil {
		return r.Generate(m, r.Config)
	}
	return generator.Generate(m, r.Config)
}

func (r *SimulationManifestReconciler) metrics() *metrics.Collector {
	if r.Metrics != nil {
		return r.Metrics
	}
	return metrics.Default
}

func (r *SimulationManifestReconciler) eventf(obj runtime.Object, eventtype, reason, msgFmt string, args ...interface{}) {
	if r.Recorder == nil {
		return
	}
	r.Recorder.Eventf(obj, eventtype, reason, msgFmt, args...)
}
