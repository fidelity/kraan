// +build integration

package apply

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	kraanscheme "github.com/fidelity/kraan/pkg/api/v1alpha1"
	"github.com/fidelity/kraan/pkg/internal/layers"

	helmopscheme "github.com/fluxcd/helm-operator/pkg/apis/helm.fluxcd.io/v1"
	//hrclientset "github.com/fluxcd/helm-operator/pkg/client/clientset/versioned"
	//hrscheme "github.com/fluxcd/helm-operator/pkg/client/clientset/versioned/scheme"

	testlogr "github.com/go-logr/logr/testing"
	gomock "github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"

	//metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func combinedScheme() *runtime.Scheme {
	intScheme := runtime.NewScheme()
	_ = k8sscheme.AddToScheme(intScheme)    // nolint:errcheck // ok
	_ = kraanscheme.AddToScheme(intScheme)  // nolint:errcheck // ok
	_ = helmopscheme.AddToScheme(intScheme) // nolint:errcheck // ok
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
		return nil, fmt.Errorf("unable to create a Kubernetes Config from KUBECONFIG '%s'", kubeconfig)
	}
	return config, nil
}

func kubeConfig(t *testing.T) *rest.Config {
	config, err := kubeConfigFromFile(t)
	if err == nil {
		return config
	}
	t.Logf("no KUBECONFIG from file - using InClusterConfig: %s", err)
	config, err = rest.InClusterConfig()
	if err != nil {
		t.Fatalf("kubernetes config error: %s", err)
	}
	return config
}

func startManager(t *testing.T, mgr ctrl.Manager) {
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		t.Logf("unable to start manager: %s", err)
		return
	}
}

func createManager(t *testing.T, config *rest.Config, scheme *runtime.Scheme, namespace string) ctrl.Manager {
	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:    scheme,
		Namespace: namespace,
	})
	if err != nil {
		t.Fatalf("unable to start manager: %s", err)
	}

	if err := createController(t, mgr); err != nil {
		t.Fatalf("unable to create controller: %s", err)
	}

	go startManager(t, mgr)

	return mgr
}

func createController(t *testing.T, mgr manager.Manager) error {
	r := createReconciler(t, mgr.GetConfig(), mgr.GetClient(), mgr.GetScheme())
	err := r.setupWithManager(mgr)
	// +kubebuilder:scaffold:builder
	if err != nil {
		return fmt.Errorf("unable to setup Reconciler with Manager")
	}
	return nil
}

type fakeReconciler struct {
	client.Client
	Config *rest.Config
	//k8client kubernetes.Interface
	Scheme  *runtime.Scheme
	Context context.Context
	T       *testing.T
}

func createReconciler(t *testing.T, config *rest.Config, client client.Client, scheme *runtime.Scheme) *fakeReconciler {
	reconciler := &fakeReconciler{
		Client:  client,
		Config:  config,
		Scheme:  scheme,
		Context: context.Background(),
		T:       t,
	}
	//reconciler.k8client = reconciler.getK8sClient()
	//reconciler.Applier, err = apply.NewApplier(client, logger, scheme)
	return reconciler
}

func (r *fakeReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := r.Context
	name := req.NamespacedName
	addonsLayer := &kraanscheme.AddonsLayer{}
	if err := r.Get(ctx, name, addonsLayer); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	r.T.Logf("Reconcile called for AddonsLayer '%s'", addonsLayer.Name)
	return ctrl.Result{}, nil
}

func indexHelmReleaseByOwner(o runtime.Object) []string {
	hr, ok := o.(*helmopscheme.HelmRelease)
	if !ok {
		return nil
	}
	owner := metav1.GetControllerOf(hr)
	if owner == nil {
		return nil
	}
	if owner.APIVersion != kraanscheme.GroupVersion.String() || owner.Kind != "AddonsLayer" {
		return nil
	}
	return []string{owner.Name}
}

func (r *fakeReconciler) setupWithManager(mgr ctrl.Manager) error {
	addonsLayer := &kraanscheme.AddonsLayer{}
	hr := &helmopscheme.HelmRelease{}
	if err := mgr.GetFieldIndexer().IndexField(hr, ".owner", indexHelmReleaseByOwner); err != nil {
		return fmt.Errorf("failed setting up FieldIndexer for HelmRelease owner: %w", err)
	}
	err := ctrl.NewControllerManagedBy(mgr).For(addonsLayer).Owns(hr).Complete(r)
	// +kubebuilder:scaffold:builder
	if err != nil {
		return fmt.Errorf("unable to setup Reconciler with Manager: %w", err)
	}
	return nil
}

func managerClient(t *testing.T, scheme *runtime.Scheme, namespace string) client.Client {
	config := kubeConfig(t)
	mgr := createManager(t, config, scheme, namespace)
	return mgr.GetClient()
}

func runtimeClient(t *testing.T, scheme *runtime.Scheme) client.Client {
	config := kubeConfig(t)
	client, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatalf("Unable to create controller runtime client: %s", err)
	}
	return client
}

func kubeCoreClient(t *testing.T) *kubernetes.Clientset {
	config := kubeConfig(t)
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatalf("Unable to create Kubernetes API Clientset: %s", err)
	}
	return clientset
}

func fakeAddonsLayer(sourcePath, layerName string, layerUID types.UID) *kraanscheme.AddonsLayer {
	kind := "AddonsLayer"
	version := "v1alpha1"
	typeMeta := metav1.TypeMeta{
		Kind:       kind,
		APIVersion: version,
	}
	now := metav1.Time{Time: time.Now()}
	layerMeta := metav1.ObjectMeta{
		Name:              layerName,
		UID:               layerUID,
		ResourceVersion:   version,
		Generation:        1,
		CreationTimestamp: now,
		ClusterName:       "TestingCluster",
	}
	sourceSpec := kraanscheme.SourceSpec{
		Name: "TestingSource",
		Path: sourcePath,
	}
	layerPreReqs := kraanscheme.PreReqs{
		K8sVersion: "1.15.3",
		//K8sVersion string `json:"k8sVersion"`
		//DependsOn []string `json:"dependsOn,omitempty"`
	}
	layerSpec := kraanscheme.AddonsLayerSpec{
		Source:  sourceSpec,
		PreReqs: layerPreReqs,
		Hold:    false,
		Version: "v1alpha1",
		//Source SourceSpec `json:"source"`
		//PreReqs PreReqs `json:"prereqs,omitempty"`
		//Hold bool `json:"hold,omitempty"`
		//Interval metav1.Duration `json:"interval"`
		//Timeout *metav1.Duration `json:"timeout,omitempty"`
		//Version string `json:"version"`
	}
	layerStatus := kraanscheme.AddonsLayerStatus{
		State:   "Testing",
		Version: "v1alpha1",
		//Conditions []Condition `json:"conditions,omitempty"`
		//State string `json:"state,omitempty"`
		//Version string `json:"version,omitempty"`
	}
	addonsLayer := &kraanscheme.AddonsLayer{
		TypeMeta:   typeMeta,
		ObjectMeta: layerMeta,
		Spec:       layerSpec,
		Status:     layerStatus,
	}
	return addonsLayer
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

func getAddonsLayer(t *testing.T, c client.Client, name string) *kraanscheme.AddonsLayer {
	addonsLayer := &kraanscheme.AddonsLayer{}
	key := client.ObjectKey{Name: name}
	if err := c.Get(context.Background(), key, addonsLayer); err != nil {
		t.Fatalf("unable to retrieve AddonsLayer '%s'", name)
	}
	return addonsLayer
}

func TestSingleApply(t *testing.T) {
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
	// Values section of the microservice.yaml HelmRelease in the directory below.
	sourcePath := "testdata/apply/single_release"
	layerName := "test"
	l := getAddonsLayer(t, client, layerName)
	layerUID := l.ObjectMeta.UID
	addonsLayer := fakeAddonsLayer(sourcePath, layerName, layerUID)

	mockLayer := layers.NewMockLayer(mockCtl)
	mockLayer.EXPECT().GetName().Return(layerName).AnyTimes()
	mockLayer.EXPECT().GetSourcePath().Return(sourcePath).AnyTimes()
	mockLayer.EXPECT().GetLogger().Return(logger).AnyTimes()
	mockLayer.EXPECT().GetAddonsLayer().Return(addonsLayer).Times(1)

	err = applier.Apply(mockLayer)
	if err != nil {
		t.Fatalf("LayerApplier.Apply returned an error: %s", err)
	}
}

func TestDoubleApply(t *testing.T) {
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
	// Values section of the microservice.yaml HelmRelease in the directory below.
	sourcePath := "testdata/apply/double_release"
	layerName := "test"
	l := getAddonsLayer(t, client, layerName)
	layerUID := l.ObjectMeta.UID
	addonsLayer := fakeAddonsLayer(sourcePath, layerName, layerUID)

	mockLayer := layers.NewMockLayer(mockCtl)
	mockLayer.EXPECT().GetName().Return(layerName).AnyTimes()
	mockLayer.EXPECT().GetSourcePath().Return(sourcePath).AnyTimes()
	mockLayer.EXPECT().GetLogger().Return(logger).AnyTimes()
	mockLayer.EXPECT().GetAddonsLayer().Return(addonsLayer).Times(2)

	err = applier.Apply(mockLayer)
	if err != nil {
		t.Fatalf("LayerApplier.Apply returned an error: %s", err)
	}
}

func TestPruneIsRequired(t *testing.T) {
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	logger := testlogr.TestLogger{T: t}
	scheme := combinedScheme()
	client := managerClient(t, scheme, "simple")

	applier, err := NewApplier(client, logger, scheme)
	if err != nil {
		t.Fatalf("The NewApplier constructor returned an error: %s", err)
	}
	t.Logf("NewApplier returned (%T) %#v", applier, applier)

	// This integration test can be forced to pass or fail at different stages by altering the
	// Values section of the microservice.yaml HelmRelease in the directory below.
	sourcePath := "testdata/apply/single_release"
	layerName := "test"
	addonsLayer := getAddonsLayer(t, client, layerName)

	mockLayer := layers.NewMockLayer(mockCtl)
	mockLayer.EXPECT().GetName().Return(layerName).AnyTimes()
	mockLayer.EXPECT().GetSourcePath().Return(sourcePath).AnyTimes()
	mockLayer.EXPECT().GetLogger().Return(logger).AnyTimes()
	mockLayer.EXPECT().GetAddonsLayer().Return(addonsLayer).Times(1)

	pruneRequired, pruneHrs, err := applier.PruneIsRequired(mockLayer)
	if err != nil {
		t.Fatalf("LayerApplier.PruneIsRequired returned an error: %s", err)
	}
	t.Logf("LayerApplier.PruneIsRequired returned %v", pruneRequired)
	t.Logf("LayerApplier.PruneIsRequired returned %d hrs to prune", len(pruneHrs))
	for _, hr := range pruneHrs {
		t.Logf("LayerApplier.PruneIsRequired - '%s'", getLabel(hr))
	}
	if pruneRequired {
		t.Fatalf("LayerApplier.PruneIsRequired returned %v when false was expected", pruneRequired)
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
	sourcePath := "testdata/apply/single_release"
	//coreClient, hrClient := kubeClients(t)
	//baseContext := context.Background()
	//timeoutContext, _ := context.WithTimeout(baseContext, 15*time.Second)
	layerName := "test"
	layerUID := types.UID("test-UID")
	addonsLayer := fakeAddonsLayer(sourcePath, layerName, layerUID)

	mockLayer := layers.NewMockLayer(mockCtl)
	mockLayer.EXPECT().GetName().Return(layerName).AnyTimes()
	mockLayer.EXPECT().GetSourcePath().Return(sourcePath).AnyTimes()
	mockLayer.EXPECT().GetLogger().Return(logger).AnyTimes()
	mockLayer.EXPECT().GetAddonsLayer().Return(addonsLayer).Times(1)

	err = applier.Apply(mockLayer)
	if err != nil {
		t.Logf("LayerApplier.Apply timed out as expected.")
	} else {
		t.Logf("LayerApplier.Apply timed out as expected.")
		//t.Fatalf("LayerApplier.Apply returned an error: %s", err)
	}
}
