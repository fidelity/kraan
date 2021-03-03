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

package controllers_test

import (
	"flag"
	"log"
	"os"
	"testing"

	helmctlv2 "github.com/fluxcd/helm-controller/api/v2beta1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	uzap "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	kind "sigs.k8s.io/kind/cmd/kind/app"
	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cmd"

	kraanv1alpha1 "github.com/fidelity/kraan/api/v1alpha1"
	"github.com/fidelity/kraan/controllers"
	"github.com/fidelity/kraan/pkg/common"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	cfg       *rest.Config         // nolint:gochecknoglobals // until we understand this code better
	k8sClient client.Client        // nolint:gochecknoglobals // until we understand this code better
	testEnv   *envtest.Environment // nolint:gochecknoglobals // until we understand this code better
)

const (
	kindClusterName = "integration-testing-cluster"
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

func startKindCluster() {
	err := os.Setenv("KIND_CLUSTER_NAME", kindClusterName)
	Expect(err).ToNot(HaveOccurred())
	p := cluster.NewProvider()
	Expect(err).ToNot(HaveOccurred())
	clusters, err := p.List()
	Expect(err).NotTo(HaveOccurred())
	if !common.ContainsString(clusters, kindClusterName) {
		args := []string{"create", "cluster"}
		err = kind.Run(cmd.NewLogger(), cmd.StandardIOStreams(), args)
		Expect(err).NotTo(HaveOccurred())
	}
}

func installHelmChart() {
	chartPath := "../chart"
	namespace := "default"
	releaseName := "kraan"

	settings := cli.New()

	actionConfig := new(action.Configuration)
	// You can pass an empty string instead of settings.Namespace() to list
	// all namespaces
	if err := actionConfig.Init(settings.RESTClientGetter(), namespace,
		os.Getenv("HELM_DRIVER"), log.Printf); err != nil {
		log.Printf("%+v", err)
		os.Exit(1)
	}

	// define values
	vals := map[string]interface{}{
		"kraan": map[string]interface{}{
			"kraanController": map[string]interface{}{
				"enabled": false,
			},
		},
	}

	vals["global"] = map[string]interface{}{
			"env": map[string]interface{}{
				"httpsProxy": "BigMaster",
			},
		}

	// load chart from the path
	chart, err := loader.Load(chartPath)
	if err != nil {
		panic(err)
	}

	client := action.NewInstall(actionConfig)
	client.Namespace = namespace
	client.ReleaseName = releaseName
	// client.DryRun = true - very handy!

	// install the chart here
	rel, err := client.Run(chart, vals)
	if err != nil {
		panic(err)
	}

	log.Printf("Installed Chart from path: %s in namespace: %s\n", rel.Name, rel.Namespace)
	// this will confirm the values set during installation
	log.Println(rel.Config)
}

var _ = BeforeSuite(func() {
	err := os.Setenv("USE_EXISTING_CLUSTER", "true")
	Expect(err).ToNot(HaveOccurred())
	startKindCluster()
	installHelmChart()

	logOpts := &zap.Options{}
	f := flag.NewFlagSet("-zap-log-level=4", flag.ExitOnError)
	logOpts.BindFlags(f)
	encCfg := uzap.NewProductionEncoderConfig()
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoder := zap.Encoder(zapcore.NewJSONEncoder(encCfg))
	logf.SetLogger(zap.New(zap.UseFlagOptions(logOpts), encoder, zap.WriteTo(GinkgoWriter)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{}

	err = kraanv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = helmctlv2.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = sourcev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	//+kubebuilder:scaffold:scheme

	/*
		A client is created for our test CRUD operations.
	*/
	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:    scheme.Scheme,
		Namespace: "",
	})
	Expect(err).ToNot(HaveOccurred())

	r, err := controllers.NewReconciler(
		k8sManager.GetConfig(),
		k8sManager.GetClient(),
		logf.Log.WithName("controller"),
		k8sManager.GetScheme())
	Expect(err).ToNot(HaveOccurred())

	err = r.SetupWithManagerAndOptions(k8sManager, controllers.AddonsLayerReconcilerOptions{
		MaxConcurrentReconciles: 1,
	})
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
	}()

	k8sClient = k8sManager.GetClient()
	Expect(k8sClient).ToNot(BeNil())
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})
