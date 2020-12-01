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

	helmctlv2 "github.com/fluxcd/helm-controller/api/v2beta1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	uzap "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	extv1b1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	_ "sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	kraanv1alpha1 "github.com/fidelity/kraan/api/v1alpha1"
	"github.com/fidelity/kraan/controllers"
	"github.com/fidelity/kraan/pkg/common"
	"github.com/fidelity/kraan/pkg/repos"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = log.Log.WithName("initialization")
)

func init() {
	log.SetLogger(zap.New())
	_ = corev1.AddToScheme(scheme)        // nolint:errcheck // ok
	_ = helmctlv2.AddToScheme(scheme)     // nolint:errcheck // ok
	_ = kraanv1alpha1.AddToScheme(scheme) // nolint:errcheck // ok
	_ = sourcev1.AddToScheme(scheme)      // nolint:errcheck // ok
	_ = extv1b1.AddToScheme(scheme)       // nolint:errcheck // ok
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

type LogLevels struct {
	Info    bool
	Debug   bool
	Trace   bool
	Highest int
}

func CheckLogLevels(log logr.Logger) LogLevels {
	lvl := LogLevels{
		Info:  log.V(0).Enabled(),
		Debug: log.V(1).Enabled(),
		Trace: log.V(2).Enabled(),
	}
	for i := 0; i < 100; i++ {
		if !log.V(i).Enabled() {
			log.V(i).Info("log-level enabled", "level", i)
			lvl.Highest = i - 1
			break
		}
	}
	return lvl
}

// NewLogger returns a logger configured the timestamps format is ISO8601
func NewLogger(logOpts *zap.Options) logr.Logger {
	encCfg := uzap.NewProductionEncoderConfig()
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoder := zap.Encoder(zapcore.NewJSONEncoder(encCfg))

	return zap.New(zap.UseFlagOptions(logOpts), encoder).WithName("kraan")
}

func main() { //nolint:funlen // ok
	var (
		metricsAddr             string
		healthAddr              string
		enableLeaderElection    bool
		leaderElectionNamespace string
		logLevel                string
		concurrent              int
		syncPeriod              time.Duration
	)

	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(
		&leaderElectionNamespace,
		"leader-election-namespace",
		"",
		"Namespace that the controller performs leader election in. defaults to namespace it is running in.",
	)

	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.IntVar(&concurrent, "concurrent", 4, "The number of concurrent reconciles per controller.")
	flag.StringVar(&logLevel, "log-level", "info", "Set logging level. Can be debug, info or error.")
	flag.StringVar(&healthAddr,
		"health-addr",
		":9440",
		"The address the health endpoint binds to.",
	)

	flag.DurationVar(
		&syncPeriod,
		"sync-period",
		time.Second*60,
		"period between reprocessing of all AddonsLayers.",
	)

	logOpts := zap.Options{}
	logOpts.BindFlags(flag.CommandLine)

	flag.Parse()

	setupLog.Info("command-line flags", "osArgs", os.Args[:1])
	logger := NewLogger(&logOpts)

	loggerType := fmt.Sprintf("%T", logger)
	lvl := CheckLogLevels(logger)
	setupLog.Info("logger configured", "loggerType", loggerType, "logLevels", lvl)

	if common.GetRuntimeNamespace() == "" {
		setupLog.Error(fmt.Errorf("RUNTIME_NAMESPACE environmental variable not set"), "please set RUNTIME_NAMESPACE environmental variable to Kraan Controller namespace")
		os.Exit(1)
	}

	mgr, err := createManager(metricsAddr, healthAddr, enableLeaderElection, leaderElectionNamespace, syncPeriod, logger)
	if err != nil {
		setupLog.Error(err, "problem creating manager")
		os.Exit(1)
	}

	r, err := controllers.NewReconciler(
		mgr.GetConfig(),
		mgr.GetClient(),
		logger.WithName("controller"),
		mgr.GetScheme())
	if err != nil {
		setupLog.Error(err, "unable to create Reconciler")
		os.Exit(1)
	}

	err = r.SetupWithManagerAndOptions(mgr, controllers.AddonsLayerReconcilerOptions{
		MaxConcurrentReconciles: concurrent,
	})
	// +kubebuilder:scaffold:builder
	if err != nil {
		setupLog.Error(err, "unable to setup Reconciler with Manager")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func createManager(metricsAddr string, healthAddr string, enableLeaderElection bool,
	leaderElectionNamespace string, syncPeriod time.Duration, logger logr.Logger) (manager.Manager, error) {
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Logger:                  logger.WithName("manager"),
		Scheme:                  scheme,
		MetricsBindAddress:      metricsAddr,
		HealthProbeBindAddress:  healthAddr,
		LeaderElection:          enableLeaderElection,
		LeaderElectionNamespace: leaderElectionNamespace,
		LeaderElectionID:        "925331a6.kraan.io",
		Namespace:               "",
		SyncPeriod:              &syncPeriod,
	})
	if err != nil {
		return nil, errors.Wrap(err, "unable to start manager")
	}

	if err := mgr.AddReadyzCheck("ping", readinessCheck); err != nil {
		return nil, errors.Wrap(err, "unable to create ready check")
	}

	if err := mgr.AddHealthzCheck("ping", livenessCheck); err != nil {
		return nil, errors.Wrap(err, "unable to create health check")
	}

	return mgr, nil
}

func readinessCheck(req *http.Request) error {
	setupLog.V(2).Info("got readiness check", "header", req.Header)
	return nil
}

func livenessCheck(req *http.Request) error {
	setupLog.V(2).Info("got liveness check", "header", req.Header)
	return nil
}
