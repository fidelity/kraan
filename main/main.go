/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	// +kubebuilder:scaffold:imports

	helmopv1 "github.com/fluxcd/helm-operator/pkg/apis/helm.fluxcd.io/v1"
	"github.com/fluxcd/pkg/runtime/logger"
	sourcev1 "github.com/fluxcd/source-controller/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	_ "sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	kraanv1alpha1 "github.com/fidelity/kraan/api/v1alpha1"
	"github.com/fidelity/kraan/controllers"
	"github.com/fidelity/kraan/pkg/repos"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = corev1.AddToScheme(scheme)        // nolint:errcheck // ok
	_ = helmopv1.AddToScheme(scheme)      // nolint:errcheck // ok
	_ = kraanv1alpha1.AddToScheme(scheme) // nolint:errcheck // ok
	_ = sourcev1.AddToScheme(scheme)      // nolint:errcheck // ok
	// +kubebuilder:scaffold:scheme

	if path, set := os.LookupEnv("DATA_PATH"); set {
		repos.DefaultRootPath = path
	}
	if host, set := os.LookupEnv("SC_HOST"); set {
		repos.DefaultHostName = host
	}
	if timeout, set := os.LookupEnv("SC_TIMEOUT"); set {
		timeOut, err := time.ParseDuration(timeout)
		if err != nil {
			setupLog.Error(err, "unable to parse timeout period")
			os.Exit(1)
		}
		repos.DefaultTimeOut = timeOut
	}
}

func main() {
	var (
		metricsAddr             string
		healthAddr              string
		enableLeaderElection    bool
		leaderElectionNamespace string
		logJSON                 bool
		logLevel                string
		syncPeriodStr           string
	)

	flag.StringVar(&metricsAddr, "metrics-addr", ":8282", "The address the metric endpoint binds to.")
	flag.StringVar(
		&leaderElectionNamespace,
		"leader-election-namespace",
		"",
		"Namespace that the controller performs leader election in. defaults to namespace it is running in.",
	)

	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&logJSON, "log-json", false, "Set logging to JSON format.")
	flag.StringVar(&logLevel, "log-level", "info", "Set logging level. Can be debug, info or error.")
	flag.StringVar(&healthAddr,
		"health-addr",
		":9440",
		"The address the health endpoint binds to.",
	)

	flag.StringVar(
		&syncPeriodStr,
		"sync-period",
		"30s",
		"period between reprocessing of all AddonsLayers.",
	)

	flag.Parse()

	syncPeriod, err := time.ParseDuration(syncPeriodStr)
	if err != nil {
		setupLog.Error(err, "unable to parse sync period")
		os.Exit(1)
	}
	ctrl.SetLogger(logger.NewLogger(logLevel, logJSON))

	mgr, err := createManager(metricsAddr, healthAddr, enableLeaderElection, leaderElectionNamespace, syncPeriod)
	if err != nil {
		setupLog.Error(err, "problem creating manager")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func createManager(metricsAddr string, healthAddr string, enableLeaderElection bool, leaderElectionNamespace string, syncPeriod time.Duration) (manager.Manager, error) {
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                  scheme,
		MetricsBindAddress:      metricsAddr,
		HealthProbeBindAddress:  healthAddr,
		LeaderElection:          enableLeaderElection,
		LeaderElectionNamespace: leaderElectionNamespace,
		LeaderElectionID:        "925331a6.kraan.io",
		Namespace:               os.Getenv("RUNTIME_NAMESPACE"),
		SyncPeriod:              &syncPeriod,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to start manager: %w", err)
	}

	if err := createController(mgr); err != nil {
		return nil, fmt.Errorf("unable to create controller: %w", err)
	}

	if err := mgr.AddReadyzCheck("ping", readinessCheck); err != nil {
		return nil, fmt.Errorf("unable to create ready check: %w", err)
	}

	if err := mgr.AddHealthzCheck("ping", livenessCheck); err != nil {
		return nil, fmt.Errorf("unable to create health check: %w", err)
	}

	return mgr, nil
}

func createController(mgr manager.Manager) error {
	r, err := controllers.NewReconciler(
		mgr.GetConfig(),
		mgr.GetClient(),
		ctrl.Log.WithName("controllers").WithName("AddonsLayer"),
		mgr.GetScheme())
	if err != nil {
		return fmt.Errorf("unable to create Reconciler: %w", err)
	}
	err = r.SetupWithManager(mgr)
	// +kubebuilder:scaffold:builder
	if err != nil {
		return fmt.Errorf("unable to setup Reconciler with Manager: %w", err)
	}
	return nil
}

func readinessCheck(req *http.Request) error {
	//setupLog.Info(fmt.Sprintf("got readiness check: %s", req.Header))
	return nil
}

func livenessCheck(req *http.Request) error {
	//setupLog.Info(fmt.Sprintf("got liveness check: %s", req.Header))
	return nil
}
