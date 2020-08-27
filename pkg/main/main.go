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

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	_ "sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	hrscheme "github.com/fluxcd/helm-operator/pkg/client/clientset/versioned/scheme"

	kraanv1alpha1 "github.com/fidelity/kraan/pkg/api/v1alpha1"
	"github.com/fidelity/kraan/pkg/controllers"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme) // nolint:errcheck // ok
	_ = kraanv1alpha1.AddToScheme(scheme)  // nolint:errcheck // ok
	_ = hrscheme.AddToScheme(scheme)       // nolint:errcheck // ok
	// +kubebuilder:scaffold:scheme
}

func main() {
	var (
		metricsAddr             string
		healthAddr              string
		enableLeaderElection    bool
		leaderElectionNamespace string
		logJSON                 bool
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

	flag.StringVar(&healthAddr,
		"health-addr",
		":9440",
		"The address the health endpoint binds to.",
	)

	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(!logJSON)))

	mgr, err := createManager(metricsAddr, healthAddr, enableLeaderElection, leaderElectionNamespace)
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

func createManager(metricsAddr string, healthAddr string, enableLeaderElection bool, leaderElectionNamespace string) (manager.Manager, error) {
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                  scheme,
		MetricsBindAddress:      metricsAddr,
		HealthProbeBindAddress:  healthAddr,
		LeaderElection:          enableLeaderElection,
		LeaderElectionNamespace: leaderElectionNamespace,
		LeaderElectionID:        "925331a6.kraan.io",
		Namespace:               os.Getenv("RUNTIME_NAMESPACE"),
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
		return fmt.Errorf("unable to setup Reconciler with Manager")
	}
	return nil
}

func readinessCheck(req *http.Request) error {
	setupLog.Info(fmt.Sprintf("got readiness check: %s", req.Header))
	return nil
}

func livenessCheck(req *http.Request) error {
	setupLog.Info(fmt.Sprintf("got liveness check: %s", req.Header))
	return nil
}
