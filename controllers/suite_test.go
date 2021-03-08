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
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"testing"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"

	helmctlv2 "github.com/fluxcd/helm-controller/api/v2beta1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"
	"github.com/ghodss/yaml"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/paulcarlton-ww/goutils/pkg/kubectl"
	"github.com/pkg/errors"
	uzap "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	coreV1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
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
	gotkSystem      = "gotk-system"
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
		err = p.Create(kindClusterName)
		Expect(err).NotTo(HaveOccurred())
		logf.Info("kind cluster crested", "cluster", kindClusterName)
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

	if len(gitopsProxy) > 0 { // nolint: nestif // ok
		httpsProxy := gitopsProxy
		if strings.ToLower(gitopsProxy) == "auto" {
			var present bool
			httpsProxy, present = os.LookupEnv("HTTPS_PROXY")
			if !present {
				httpsProxy, present = os.LookupEnv("https_proxy")
				if !present {
					httpsProxy, _ = os.LookupEnv("HTTP_PROXY")
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
	}

	return values
}

func createImagePullSecret(log logr.Logger, namespace string) {
	imagePullSecretName := os.Getenv("IMAGE_PULL_SECRET_NAME")

	if len(imagePullSecretName) == 0 {
		return
	}

	source := os.Getenv("IMAGE_PULL_SECRET_SOURCE")
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
	getOptions := v1.GetOptions{}
	secret, e := secretsClient.Get(context.Background(), "gotk-regcred", getOptions)
	if e != nil {
		if !k8serrors.IsNotFound(e) {
			Expect(err).ToNot(HaveOccurred())
		}
	} else {
		log.Info("got secret", "name", secret.Name, "namespace", secret.Namespace)
		return
	}
	options := v1.CreateOptions{}
	_, err = secretsClient.Create(context.Background(), &secretSpec, options)
	Expect(err).ToNot(HaveOccurred())
}

func getRestClient() *rest.Config {
	// creates the clientset
	kubeConfig, present := os.LookupEnv("KUBECONFIG")
	if !present {
		kubeConfig = os.Getenv("HOME") + "/.kube/config"
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	Expect(err).ToNot(HaveOccurred())

	return config
}

func getK8sClient() kubernetes.Interface {
	clientset, err := kubernetes.NewForConfig(getRestClient())
	Expect(err).ToNot(HaveOccurred())

	return clientset
}

func debugLogf(format string, values ...interface{}) {
	log.Info("helm debugging", "message", fmt.Sprintf(format, values...))
}

func isReleasePresent(chartName, namespace string, actionConfig *action.Configuration) bool {
	listClient := action.NewList(actionConfig)
	listClient.All = true
	listClient.AllNamespaces = true
	releases, err := listClient.Run()
	Expect(err).ToNot(HaveOccurred())
	for _, release := range releases {
		if release.Name == chartName && release.Namespace == namespace {
			return true
		}
	}
	return false
}

func installHelmChart(logf logr.Logger, releaseName, namespace string, actionConfig *action.Configuration, chart *chart.Chart) {
	client := action.NewInstall(actionConfig)
	client.CreateNamespace = true
	client.Namespace = namespace
	client.ReleaseName = releaseName

	// install the chart here
	rel, err := client.Run(chart, setValues(namespace))
	Expect(err).ToNot(HaveOccurred())

	logf.Info("Installed Chart", "path", rel.Name, "namespace", rel.Namespace)
	logf.Info("Chart values overridden", "values", rel.Config)
}

func upgradeHelmChart(logf logr.Logger, releaseName, namespace string, actionConfig *action.Configuration, chart *chart.Chart) {
	client := action.NewUpgrade(actionConfig)
	client.Namespace = namespace

	// install the chart here
	rel, err := client.Run(releaseName, chart, setValues(namespace))
	Expect(err).ToNot(HaveOccurred())

	logf.Info("Upgraded Chart", "path", rel.Name, "namespace", rel.Namespace)
	logf.Info("Chart values overridden", "values", rel.Config)
}

func deployHelmChart(logf logr.Logger, namespace string) {
	chartPath := "../chart"

	settings := cli.New()

	actionConfig := new(action.Configuration)
	// You can pass an empty string instead of settings.Namespace() to list
	// all namespaces
	err := actionConfig.Init(settings.RESTClientGetter(), namespace,
		os.Getenv("HELM_DRIVER"), debugLogf)
	Expect(err).ToNot(HaveOccurred())

	// load chart from the path
	chart, err := loader.Load(chartPath)
	Expect(err).ToNot(HaveOccurred())

	releaseName := chart.Name()

	if isReleasePresent(releaseName, namespace, actionConfig) {
		upgradeHelmChart(log, releaseName, namespace, actionConfig, chart)
	} else {
		installHelmChart(log, releaseName, namespace, actionConfig, chart)
	}
	createImagePullSecret(log, namespace)
}

func applySetupYAML(log logr.Logger) {
	kubeCtl, err := kubectl.NewKubectl(log)
	Expect(err).ToNot(HaveOccurred())
	cmd := kubeCtl.Apply("./testdata/setup")
	output, e := cmd.Run()
	Expect(e).ToNot(HaveOccurred())

	log.Info("applied", "apply response", string(output))
}

type portForwardPodRequest struct {
	// RestConfig is the kubernetes config
	RestConfig *rest.Config
	// Pod is the selected pod for this port forwarding
	Pod coreV1.Pod
	// LocalPort is the local port that will be selected to expose the PodPort
	LocalPort int
	// PodPort is the target port for the pod
	PodPort int
	// Steams configures where to write or read input from
	Streams genericclioptions.IOStreams
	// StopCh is the channel used to manage the port forward lifecycle
	StopCh <-chan struct{}
	// ReadyCh communicates when the tunnel is ready to receive traffic
	ReadyCh chan struct{}
}

func portForwardPod(req portForwardPodRequest) error {
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward",
		req.Pod.Namespace, req.Pod.Name)
	hostIP := strings.TrimLeft(req.RestConfig.Host, "htps:/")

	transport, upgrader, err := spdy.RoundTripperFor(req.RestConfig)
	Expect(err).ToNot(HaveOccurred())

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, &url.URL{Scheme: "https", Path: path, Host: hostIP})
	fw, err := portforward.New(dialer, []string{fmt.Sprintf("%d:%d", req.LocalPort, req.PodPort)}, req.StopCh, req.ReadyCh, req.Streams.Out, req.Streams.ErrOut)
	Expect(err).ToNot(HaveOccurred())

	return fw.ForwardPorts()
}

func getSouceControllerPodName(namespace string) string {
	listOptions := &client.ListOptions{}
	pods, err := client.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
	k8sClient.List(context.Background(), pod, listOptions)
}

func portForward(namespace string) {
	var wg sync.WaitGroup
	wg.Add(1)

	// stopCh control the port forwarding lifecycle. When it gets closed the
	// port forward will terminate
	stopCh := make(chan struct{}, 1)
	// readyCh communicate when the port forward is ready to get traffic
	readyCh := make(chan struct{})
	// stream is used to tell the port forwarder where to place its output or
	// where to expect input if needed. For the port forwarding we just need
	// the output eventually
	stream := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}

	// managing termination signal from the terminal. As you can see the stopCh
	// gets closed to gracefully handle its termination.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		fmt.Println("Bye...")
		close(stopCh)
		wg.Done()
	}()

	go func() {
		// PortForward the pod specified from its port 9090 to the local port
		// 8080
		err := portForwardPod(portForwardPodRequest{
			RestConfig: getRestClient(),
			Pod: coreV1.Pod{
				ObjectMeta: v1.ObjectMeta{
					Name:      getSouceControllerPodName(),
					Namespace: namespace,
				},
			},
			LocalPort: 8090,
			PodPort:   9090,
			Streams:   stream,
			StopCh:    stopCh,
			ReadyCh:   readyCh,
		})
		if err != nil {
			panic(err)
		}
	}()

	select {
	case <-readyCh:
		break
	}
	println("Port forwarding to source controller is ready")
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

	namespace, present := os.LookupEnv("KRAAN_NAMESPACE")
	if !present {
		namespace = gotkSystem
	} else {
		log.Error(errNotYetSupported, "kraan namespace selection not yet supported")
	}
	Expect(namespace).To(MatchRegexp(gotkSystem))

	deployHelmChart(log, namespace)
	applySetupYAML(log)

	portForward(namespace)

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
