// virtualization-framework operator — reconciles SimulationManifest into Istio resources.
package main

import (
	"flag"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	simv1 "github.com/servicemesh/virtualization-framework/api/simulation/v1alpha1"
	"github.com/servicemesh/virtualization-framework/internal/admission"
	"github.com/servicemesh/virtualization-framework/internal/config"
	"github.com/servicemesh/virtualization-framework/internal/controller"
	"github.com/servicemesh/virtualization-framework/internal/metrics"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(simv1.AddToScheme(scheme))
}

func main() {
	var metricsAddr string
	var probeAddr string
	var enableLeaderElection bool
	var webhookEnabled bool
	var webhookCertDir string
	var webhookPort int
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "metrics endpoint")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "health probe endpoint")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "enable leader election")
	flag.BoolVar(&webhookEnabled, "webhook-enabled", true, "serve validating admission webhook")
	flag.StringVar(&webhookCertDir, "webhook-cert-dir", "/tmp/k8s-webhook-server/serving-certs", "TLS cert directory for webhook")
	flag.IntVar(&webhookPort, "webhook-port", 9443, "webhook listen port")
	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	setupLog := ctrl.Log.WithName("setup")

	cfg := config.FromEnv()
	setupLog.Info("starting virtualization-framework",
		"environment", cfg.Environment,
		"microcks", cfg.DefaultMicrocksHostPort,
		"webhookEnabled", webhookEnabled,
	)

	metrics.Default.MustRegister(crmetrics.Registry)

	mgrOpts := ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "virtualization-framework.simulation.io",
	}
	if webhookEnabled {
		mgrOpts.WebhookServer = webhook.NewServer(webhook.Options{
			Port:    webhookPort,
			CertDir: webhookCertDir,
		})
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), mgrOpts)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	reconciler := &controller.SimulationManifestReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Config:   cfg,
		Recorder: mgr.GetEventRecorderFor("virtualization-framework"),
		Metrics:  metrics.Default,
	}
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&simv1.SimulationManifest{}).
		Complete(reconciler); err != nil {
		setupLog.Error(err, "unable to create controller")
		os.Exit(1)
	}

	if webhookEnabled {
		if err := ctrl.NewWebhookManagedBy(mgr).
			For(&simv1.SimulationManifest{}).
			WithDefaulter(&admission.Defaulter{Config: cfg}).
			WithValidator(&admission.Validator{Config: cfg}).
			Complete(); err != nil {
			setupLog.Error(err, "unable to create webhooks")
			os.Exit(1)
		}
		setupLog.Info("mutating+validating webhooks registered", "certDir", webhookCertDir, "port", webhookPort)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "healthz")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "readyz")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "manager stopped")
		os.Exit(1)
	}
}
