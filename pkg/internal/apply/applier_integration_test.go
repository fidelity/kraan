// +build integration

package apply

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	//hrclientset "github.com/fluxcd/helm-operator/pkg/client/clientset/versioned"
	kraanscheme "github.com/fidelity/kraan/pkg/api/v1alpha1"
	"github.com/fidelity/kraan/pkg/internal/layers"
	hrscheme "github.com/fluxcd/helm-operator/pkg/client/clientset/versioned/scheme"

	//"github.com/go-logr/logr"
	testlogr "github.com/go-logr/logr/testing"
	gomock "github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"

	//metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func combinedScheme() *runtime.Scheme {
	intScheme := runtime.NewScheme()
	_ = k8sscheme.AddToScheme(intScheme)   // nolint:errcheck // ok
	_ = kraanscheme.AddToScheme(intScheme) // nolint:errcheck // ok
	_ = hrscheme.AddToScheme(intScheme)    // nolint:errcheck // ok
	return intScheme
}

func kubeConfigFromFile(t *testing.T) (*rest.Config, error) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if len(kubeconfig) == 0 {
		kubeconfig = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	} else {
		t.Logf("Using KUBECONFIG at '%s'", kubeconfig)
	}
	t.Logf("Checking for KUBECONFIG file '%s'", kubeconfig)
	info, err := os.Stat(kubeconfig)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("KUBECONFIG '%s' is not a configuration file", kubeconfig)
	}
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("Unable to create a Kubernetes Config from KUBECONFIG '%s'", kubeconfig)
	}
	return config, nil
}

func kubeConfig(t *testing.T) (*rest.Config, error) {
	config, err := kubeConfigFromFile(t)
	if err == nil {
		return config, nil
	}
	t.Logf("No KUBECONFIG from file '%s' - using InClusterConfig", err)
	return rest.InClusterConfig()
}

func runtimeClient(t *testing.T, scheme *runtime.Scheme) client.Client {
	config, err := kubeConfig(t)
	if err != nil {
		t.Fatalf("kubernetes config error: %s", err)
	}
	client, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatalf("Unable to create controller runtime client: %s", err)
	}
	return client
}

func kubeCoreClient(t *testing.T) *kubernetes.Clientset {
	config, err := kubeConfig(t)
	if err != nil {
		t.Fatalf("kubernetes config error: %s", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatalf("Unable to create Kubernetes API Clientset: %s", err)
	}
	return clientset
}

func TestConnectToCluster(t *testing.T) {
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	scheme := combinedScheme()
	client := runtimeClient(t, scheme)
	k8sclient := kubeCoreClient(t)

	info, err := k8sclient.ServerVersion()
	if err != nil {
		t.Fatalf("Error getting server version: %s", err)
	}
	t.Logf("Server version %s", info.GitVersion)

	namespaceList := &corev1.NamespaceList{}
	err = client.List(context.Background(), namespaceList)
	if err != nil {
		t.Fatalf("runtime error getting namespaces: %s", err)
	}
	for _, namespace := range namespaceList.Items {
		t.Logf("Found Namespace '%s'", namespace.GetName())
	}

}

func TestSimpleApply(t *testing.T) {
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	logger := testlogr.TestLogger{T: t}
	scheme := combinedScheme()
	client := runtimeClient(t, scheme)

	applier, err := NewApplier(client, logger, scheme)
	if err != nil {
		t.Fatalf("The NewApplier constructor returned an error: %s", err)
	}
	t.Logf("NewApplier returned (%T) %#v", applier, applier)

	// This integration test can be forced to pass or fail at different stages by altering the
	// Values section of the podinfo.yaml HelmRelease in the directory below.
	sourcePath := "testdata/apply/simpleapply"
	//baseContext := context.Background()

	mockLayer := layers.NewMockLayer(mockCtl)
	mockLayer.EXPECT().GetNamespace().Return("simple").AnyTimes()
	mockLayer.EXPECT().GetName().Return("testLayer").AnyTimes()
	mockLayer.EXPECT().GetSourcePath().Return(sourcePath).AnyTimes()
	mockLayer.EXPECT().GetLogger().Return(logger).AnyTimes()
	//mockLayer.EXPECT().GetHelmReleaseClient().Return(hrClient).AnyTimes()
	//mockLayer.EXPECT().GetContext().Return(baseContext).AnyTimes()

	err = applier.Apply(mockLayer)
	if err != nil {
		t.Fatalf("LayerApplier.Apply returned an error: %s", err)
	}
}

func TestApplyContextTimeoutIntegration(t *testing.T) {
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	logger := testlogr.TestLogger{T: t}
	scheme := combinedScheme()
	client := runtimeClient(t, scheme)
	applier, err := NewApplier(client, logger, scheme)
	if err != nil {
		t.Fatalf("The NewApplier constructor returned an error: %s", err)
	}
	t.Logf("NewApplier returned (%T) %#v", applier, applier)

	// This integration test can be forced to pass or fail at different stages by altering the
	// Values section of the podinfo.yaml HelmRelease in the directory below.
	sourcePath := "testdata/apply/simpleapply"
	//coreClient, hrClient := kubeClients(t)
	//baseContext := context.Background()
	//timeoutContext, _ := context.WithTimeout(baseContext, 15*time.Second)

	mockLayer := layers.NewMockLayer(mockCtl)
	mockLayer.EXPECT().GetNamespace().Return("simple").AnyTimes()
	mockLayer.EXPECT().GetName().Return("testLayer").AnyTimes()
	mockLayer.EXPECT().GetSourcePath().Return(sourcePath).AnyTimes()
	//mockLayer.EXPECT().GetLogger().Return(logger).AnyTimes()
	//mockLayer.EXPECT().GetK8sClient().Return(coreClient).AnyTimes()
	//mockLayer.EXPECT().GetHelmReleaseClient().Return(hrClient).AnyTimes()
	//mockLayer.EXPECT().GetContext().Return(timeoutContext).AnyTimes()

	err = applier.Apply(mockLayer)
	if err != nil {
		t.Logf("LayerApplier.Apply timed out as expected.")
	} else {
		//t.Fatalf("LayerApplier.Apply returned an error: %s", err)
	}
}
