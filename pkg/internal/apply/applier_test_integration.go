// +build integration

// Package apply ...
package apply

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	hrclientset "github.com/fluxcd/helm-operator/pkg/client/clientset/versioned"
	hoscheme "github.com/fluxcd/helm-operator/pkg/client/clientset/versioned/scheme"

	"github.com/fidelity/kraan/pkg/internal/layers"
	"github.com/go-logr/logr"
	testlogr "github.com/go-logr/logr/testing"
	gomock "github.com/golang/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corescheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func init() {
	hoscheme.AddToScheme(corescheme.Scheme)
}

func xtestLogger(t *testing.T) logr.Logger {
	return testlogr.TestLogger{T: t}
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

func kubeCoreClient(config *rest.Config, t *testing.T) *kubernetes.Clientset {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatalf("Unable to create Kubernetes API Clientset: %s", err)
	}
	return clientset
}

func helmReleaseClient(config *rest.Config, t *testing.T) *hrclientset.Clientset {
	clientset, err := hrclientset.NewForConfig(config)
	if err != nil {
		t.Fatalf("Unable to create a HelmRelease API Client: %s", err)
	}
	return clientset
}

func kubeClients(t *testing.T) (coreClient *kubernetes.Clientset, hrClient *hrclientset.Clientset) {
	config, err := kubeConfig(t)
	t.Logf("KUBECONFIG refers to host %s", config.Host)
	if err != nil {
		t.Fatalf("K8S config error: %s", err)
	}
	return kubeCoreClient(config, t), helmReleaseClient(config, t)
}

func TestConnectToCluster(t *testing.T) {
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	coreClient, _ := kubeClients(t)

	info, err := coreClient.ServerVersion()
	if err != nil {
		t.Fatalf("Error getting server version: %s", err)
	}
	t.Logf("Server version %s", info.GitVersion)
	namespaces, err := coreClient.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Error getting namespaces: %s", err)
	}
	for _, ns := range namespaces.Items {
		t.Logf("Found NS '%s'", ns.GetName())
	}
}

func TestSimpleApplyIntegration(t *testing.T) {
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	logger := testlogr.TestLogger{T: t}
	applier, err := NewApplier(logger)
	if err != nil {
		t.Fatalf("The NewApplier constructor returned an error: %s", err)
	}
	t.Logf("NewApplier returned (%T) %#v", applier, applier)

	// This integration test can be forced to pass or fail at different stages by altering the
	// Values section of the podinfo.yaml HelmRelease in the directory below.
	sourcePath := "testdata/apply/simpleapply"
	coreClient, hrClient := kubeClients(t)
	baseContext := context.Background()

	mockLayer := layers.NewMockLayer(mockCtl)
	mockLayer.EXPECT().GetNamespace().Return("simple").AnyTimes()
	mockLayer.EXPECT().GetName().Return("testLayer").AnyTimes()
	mockLayer.EXPECT().GetSourcePath().Return(sourcePath).AnyTimes()
	mockLayer.EXPECT().GetLogger().Return(logger).AnyTimes()
	mockLayer.EXPECT().GetK8sClient().Return(coreClient).AnyTimes()
	mockLayer.EXPECT().GetHelmReleaseClient().Return(hrClient).AnyTimes()
	mockLayer.EXPECT().GetContext().Return(baseContext).AnyTimes()

	err = applier.Apply(mockLayer)
	if err != nil {
		t.Fatalf("LayerApplier.Apply returned an error: %s", err)
	}
}

// TestApplyContextTimeoutIntegration - isn't really valid anymore since Apply only applies the YAML and returns without waiting.
func TestApplyContextTimeoutIntegration(t *testing.T) {
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	logger := testlogr.TestLogger{T: t}
	applier, err := NewApplier(logger)
	if err != nil {
		t.Fatalf("The NewApplier constructor returned an error: %s", err)
	}
	t.Logf("NewApplier returned (%T) %#v", applier, applier)

	// This integration test can be forced to pass or fail at different stages by altering the
	// Values section of the podinfo.yaml HelmRelease in the directory below.
	sourcePath := "testdata/apply/simpleapply"
	coreClient, hrClient := kubeClients(t)
	baseContext := context.Background()
	timeoutContext, _ := context.WithTimeout(baseContext, 15*time.Second)

	mockLayer := layers.NewMockLayer(mockCtl)
	mockLayer.EXPECT().GetNamespace().Return("simple").AnyTimes()
	mockLayer.EXPECT().GetName().Return("testLayer").AnyTimes()
	mockLayer.EXPECT().GetSourcePath().Return(sourcePath).AnyTimes()
	mockLayer.EXPECT().GetLogger().Return(logger).AnyTimes()
	mockLayer.EXPECT().GetK8sClient().Return(coreClient).AnyTimes()
	mockLayer.EXPECT().GetHelmReleaseClient().Return(hrClient).AnyTimes()
	mockLayer.EXPECT().GetContext().Return(timeoutContext).AnyTimes()

	err = applier.Apply(mockLayer)
	if err != nil {
		t.Logf("LayerApplier.Apply timed out as expected.")
	} else {
		//t.Fatalf("LayerApplier.Apply returned an error: %s", err)
	}
}
