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
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	helmctlv2 "github.com/fluxcd/helm-controller/api/v2beta1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"
	"github.com/ghodss/yaml"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/paulcarlton-ww/goutils/pkg/kubectl"
	"github.com/paulcarlton-ww/goutils/pkg/logging"
	"github.com/pkg/errors"
	uzap "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	coreV1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubectl/pkg/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/kind/pkg/cluster"

	kraanv1alpha1 "github.com/fidelity/kraan/api/v1alpha1"
	"github.com/fidelity/kraan/controllers"
	"github.com/fidelity/kraan/pkg/common"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	cfg                *rest.Config                      // nolint:gochecknoglobals // until we understand this code better
	k8sClient          client.Client                     // nolint:gochecknoglobals // until we understand this code better
	testEnv            *envtest.Environment              // nolint:gochecknoglobals // until we understand this code better
	log                logr.Logger                       // nolint:gochecknoglobals // needed for debugLog
	errNotYetSupported = errors.New("not yet supported") // nolint:gochecknoglobals // ok
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

func startKindCluster(logf logr.Logger) {
	err := os.Setenv("KIND_CLUSTER_NAME", kindClusterName)
	Expect(err).ToNot(HaveOccurred())
	p := cluster.NewProvider()
	Expect(err).ToNot(HaveOccurred())
	clusters, err := p.List()
	Expect(err).NotTo(HaveOccurred())
	if !common.ContainsString(clusters, kindClusterName) {
		//args := []string{"create", "cluster"}
		//err = kind.Run(cmd.NewLogger(), cmd.StandardIOStreams(), args)
		err = p.Create(kindClusterName)
		Expect(err).NotTo(HaveOccurred())
	} else {
		logf.Info("using existing kind cluster", "cluster", kindClusterName)
	}
}

func setValues(namespace string) map[string]interface{} {
	// disable kraan controller deployment
	values := map[string]interface{}{
		"kraan": map[string]interface{}{
			"kraanController": map[string]interface{}{
				"enabled": false,
			},
		},
	}

	gitopsProxy := os.Getenv("GITOPS_USE_PROXY")

	if len(gitopsProxy) > 0 {
		httpsProxy := gitopsProxy
		if strings.ToLower(gitopsProxy) == "auto" {
			var present bool
			httpsProxy, present = os.LookupEnv("HTTPS_PROXY")
			if !present {
				httpsProxy, present = os.LookupEnv("https_proxy")
				if !present {
					httpsProxy, present = os.LookupEnv("HTTP_PROXY")
				}
			}
		}
		values["global"] = map[string]interface{}{
			"env": map[string]interface{}{
				"httpsProxy": httpsProxy,
			},
		}
	}

	imagePullSecretName := os.Getenv("IMAGE_PULL_SECRET_NAME")

	if len(imagePullSecretName) > 0 {
		values["gitops"] = map[string]interface{}{
			"soureController": map[string]interface{}{
				"imagePullSecret": map[string]interface{}{
					"name": imagePullSecretName,
				},
			},

			"helmController": map[string]interface{}{
				"imagePullSecret": map[string]interface{}{
					"name": imagePullSecretName,
				},
			},
		}
		createImagePullSecret(namespace, imagePullSecretName, os.Getenv("IMAGE_PULL_SECRET_SOURCE"))
	}

	return values
}

func createImagePullSecret(namespace, name, source string) {
	var secretData []byte
	var err error
	if strings.ToLower(source) != "auto" {
		secretData, err = ioutil.ReadFile(source)
		Expect(err).ToNot(HaveOccurred())
	} else {
		log.Error(errNotYetSupported, "auto generation of image pull secret not yet supported")
	}
	Expect(secretData).ToNot(BeNil())

	var secretSpec coreV1.Secret
	err = yaml.Unmarshal(secretData, &secretSpec)
	Expect(err).ToNot(HaveOccurred())

	secretsClient := getK8sClient().CoreV1().Secrets(namespace)
	options := v1.CreateOptions{}
	_, err = secretsClient.Create(context.Background(), &secretSpec, options)
	Expect(err).ToNot(HaveOccurred())

}

func getK8sClient() kubernetes.Interface {
	// creates the clientset
	kubeConfig, present := os.LookupEnv("KUBECONFIG")
	if !present {
		kubeConfig = os.Getenv("HOME") + "/.kube/config"
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	Expect(err).ToNot(HaveOccurred())

	clientset, err := kubernetes.NewForConfig(config)
	Expect(err).ToNot(HaveOccurred())

	return clientset
}
func debugLog(format string, values ...interface{}) {
	log.Info("helm debugging", "message", fmt.Sprintf(format, values...))
}

func installHelmChart(logf logr.Logger) {
	chartPath := "../chart"
	namespace := "gotk-system"
	releaseName := "kraan"

	settings := cli.New()

	actionConfig := new(action.Configuration)
	// You can pass an empty string instead of settings.Namespace() to list
	// all namespaces
	err := actionConfig.Init(settings.RESTClientGetter(), namespace,
		os.Getenv("HELM_DRIVER"), debugLog)
	Expect(err).ToNot(HaveOccurred())

	// load chart from the path
	chart, err := loader.Load(chartPath)
	Expect(err).ToNot(HaveOccurred())

	client := action.NewInstall(actionConfig)
	client.Namespace = namespace
	client.ReleaseName = releaseName

	// install the chart here
	rel, err := client.Run(chart, setValues(namespace))
	Expect(err).ToNot(HaveOccurred())

	logf.Info("Installed Chart", "path", rel.Name, "namespace", rel.Namespace)
	// this will confirm the values set during installation
	logf.Info("Chart values overridden", "values", rel.Config)
}

func applySetupYAML(log logr.Logger) {
	kubeCtl, err := kubectl.NewKubectl(log)
	Expect(err).ToNot(HaveOccurred())
	cmd := kubeCtl.Apply("./testdata/setup")
	output, e := cmd.Run()
	Expect(e).ToNot(HaveOccurred())

	log.Info("applied", "apply response", logging.LogJSON(output))
}

var _ = BeforeSuite(func() {
	err := os.Setenv("USE_EXISTING_CLUSTER", "true")
	Expect(err).ToNot(HaveOccurred())

	logOpts := &zap.Options{}
	f := flag.NewFlagSet("-zap-log-level=4", flag.ExitOnError)
	logOpts.BindFlags(f)
	encCfg := uzap.NewProductionEncoderConfig()
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoder := zap.Encoder(zapcore.NewJSONEncoder(encCfg))
	log = zap.New(zap.UseFlagOptions(logOpts), encoder, zap.WriteTo(GinkgoWriter))
	logf.SetLogger(log)

	startKindCluster(log)
	applySetupYAML(log)
	installHelmChart(log)
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
