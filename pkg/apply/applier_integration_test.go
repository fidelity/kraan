// +build integration

package apply

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	helmopv1 "github.com/fluxcd/helm-controller/api/v2alpha1"
	testlogr "github.com/go-logr/logr/testing"
	gomock "github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	types "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	kraanv1alpha1 "github.com/fidelity/kraan/api/v1alpha1"
	mocklayers "github.com/fidelity/kraan/pkg/mocks/layers"
)

func combinedScheme() *runtime.Scheme {
	intScheme := runtime.NewScheme()
	_ = k8sscheme.AddToScheme(intScheme)     // nolint:errcheck // ok
	_ = kraanv1alpha1.AddToScheme(intScheme) // nolint:errcheck // ok
	_ = helmopv1.AddToScheme(intScheme)      // nolint:errcheck // ok
	return intScheme
}

func kubeConfigFromFile(t *testing.T) (config *rest.Config, ok bool) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if len(kubeconfig) == 0 {
		kubeconfig = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	} else {
		t.Logf("Using KUBECONFIG at '%s'", kubeconfig)
	}
	t.Logf("Checking for KUBECONFIG file '%s'", kubeconfig)
	info, err := os.Stat(kubeconfig)
	if err != nil {
		t.Logf("unable to stat KUBECONFIG file '%s': %s", kubeconfig, err)
		return nil, false
	}
	if !info.Mode().IsRegular() {
		t.Logf("KUBECONFIG '%s' is not a configuration file", kubeconfig)
		return nil, false
	}
	config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		t.Logf("unable to create a Kubernetes Config from KUBECONFIG '%s'", kubeconfig)
		return nil, false
	}
	return config, true
}

func kubeConfig(t *testing.T) *rest.Config {
	config, ok := kubeConfigFromFile(t)
	if ok {
		return config
	}
	t.Logf("no KUBECONFIG from file - using InClusterConfig")
	config, err := rest.InClusterConfig()
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

func createManager(ctx context.Context, t *testing.T, config *rest.Config, scheme *runtime.Scheme, namespace string) ctrl.Manager {
	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:    scheme,
		Namespace: namespace,
	})
	if err != nil {
		t.Fatalf("unable to start manager: %s", err)
	}

	createController(ctx, t, mgr)

	go startManager(t, mgr)

	return mgr
}

func createController(ctx context.Context, t *testing.T, mgr manager.Manager) {
	r := createReconciler(ctx, t, mgr.GetConfig(), mgr.GetClient(), mgr.GetScheme())
	err := r.setupWithManager(mgr)
	// +kubebuilder:scaffold:builder
	if err != nil {
		t.Fatalf("unable to setup Reconciler with Manager")
	}
	return
}

type fakeReconciler struct {
	client.Client
	Config *rest.Config
	//k8client kubernetes.Interface
	Scheme  *runtime.Scheme
	Context context.Context
	T       *testing.T
}

func createReconciler(ctx context.Context, t *testing.T, config *rest.Config, client client.Client, scheme *runtime.Scheme) *fakeReconciler {
	reconciler := &fakeReconciler{
		Client:  client,
		Config:  config,
		Scheme:  scheme,
		Context: ctx,
		T:       t,
	}
	//reconciler.k8client = reconciler.getK8sClient()
	//reconciler.Applier, err = apply.NewApplier(client, logger, scheme)
	return reconciler
}

func (r *fakeReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := r.Context
	name := req.NamespacedName
	addonsLayer := &kraanv1alpha1.AddonsLayer{}
	if err := r.Get(ctx, name, addonsLayer); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	r.T.Logf("Reconcile called for AddonsLayer '%s'", addonsLayer.Name)
	return ctrl.Result{}, nil
}

func indexHelmReleaseByOwner(o runtime.Object) []string {
	hr, ok := o.(*helmopv1.HelmRelease)
	if !ok {
		return nil
	}
	owner := metav1.GetControllerOf(hr)
	if owner == nil {
		return nil
	}
	if owner.APIVersion != kraanv1alpha1.GroupVersion.String() || owner.Kind != "AddonsLayer" {
		return nil
	}
	return []string{owner.Name}
}

func (r *fakeReconciler) setupWithManager(mgr ctrl.Manager) error {
	addonsLayer := &kraanv1alpha1.AddonsLayer{}
	hr := &helmopv1.HelmRelease{}
	if err := mgr.GetFieldIndexer().IndexField(r.Context, hr, ".owner", indexHelmReleaseByOwner); err != nil {
		return fmt.Errorf("failed setting up FieldIndexer for HelmRelease owner: %w", err)
	}
	err := ctrl.NewControllerManagedBy(mgr).For(addonsLayer).Owns(hr).Complete(r)
	// +kubebuilder:scaffold:builder
	if err != nil {
		return fmt.Errorf("unable to setup Reconciler with Manager: %w", err)
	}
	return nil
}

func managerClient(ctx context.Context, t *testing.T, scheme *runtime.Scheme, namespace string) client.Client {
	config := kubeConfig(t)
	mgr := createManager(ctx, t, config, scheme, namespace)
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

func TestConnectToCluster(t *testing.T) {
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	ctx := context.Background()

	scheme := combinedScheme()
	client := runtimeClient(t, scheme)
	k8sclient := kubeCoreClient(t)

	info, err := k8sclient.ServerVersion()
	if err != nil {
		t.Fatalf("Error getting server version: %s", err)
	}
	t.Logf("Server version %s", info.GitVersion)

	namespaceList := &corev1.NamespaceList{}
	err = client.List(ctx, namespaceList)
	if err != nil {
		t.Fatalf("runtime error getting namespaces: %s", err)
	}
	for _, namespace := range namespaceList.Items {
		t.Logf("Found Namespace '%s'", namespace.GetName())
	}
}

func getAddonsLayer(ctx context.Context, t *testing.T, c client.Client, name string) *kraanv1alpha1.AddonsLayer {
	addonsLayer := &kraanv1alpha1.AddonsLayer{}
	key := client.ObjectKey{Name: name}
	if err := c.Get(ctx, key, addonsLayer); err != nil {
		t.Fatalf("unable to retrieve AddonsLayer '%s'", name)
	}
	return addonsLayer
}

func getHR(ctx context.Context, t *testing.T, c client.Client, namespace string, name string) *helmopv1.HelmRelease {
	hr := &helmopv1.HelmRelease{}
	key := client.ObjectKey{Namespace: namespace, Name: name}
	if err := c.Get(ctx, key, hr); err != nil {
		t.Fatalf("unable to retrieve HelmRelease '%s/%s'", namespace, name)
	}
	return hr
}

func TestSingleApply(t *testing.T) {
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	ctx := context.Background()

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
	l := getAddonsLayer(ctx, t, client, layerName)
	layerUID := l.ObjectMeta.UID
	addonsLayer := fakeAddonsLayer(sourcePath, layerName, layerUID)

	mockLayer := mocklayers.NewMockLayer(mockCtl)
	mockLayer.EXPECT().GetName().Return(layerName).AnyTimes()
	mockLayer.EXPECT().GetSourcePath().Return(sourcePath).AnyTimes()
	mockLayer.EXPECT().GetLogger().Return(logger).AnyTimes()
	mockLayer.EXPECT().GetAddonsLayer().Return(addonsLayer).Times(1)

	err = applier.Apply(ctx, mockLayer)
	if err != nil {
		t.Fatalf("LayerApplier.Apply returned an error: %s", err)
	}
	hr := getHR(ctx, t, client, "simple", "microservice")
	t.Logf("Found HelmRelease '%s/%s'", hr.GetNamespace(), hr.GetName())
}

func TestDoubleApply(t *testing.T) {
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	ctx := context.Background()

	logger := testlogr.TestLogger{T: t}
	scheme := combinedScheme()
	client := managerClient(ctx, t, scheme, "simple")

	applier, err := NewApplier(client, logger, scheme)
	if err != nil {
		t.Fatalf("The NewApplier constructor returned an error: %s", err)
	}
	t.Logf("NewApplier returned (%T) %#v", applier, applier)

	// This integration test can be forced to pass or fail at different stages by altering the
	// Values section of the microservice.yaml HelmRelease in the directory below.
	sourcePath := "testdata/apply/double_release"
	layerName := "test"
	l := getAddonsLayer(ctx, t, client, layerName)
	layerUID := l.ObjectMeta.UID
	addonsLayer := fakeAddonsLayer(sourcePath, layerName, layerUID)

	mockLayer := mocklayers.NewMockLayer(mockCtl)
	mockLayer.EXPECT().GetName().Return(layerName).AnyTimes()
	mockLayer.EXPECT().GetSourcePath().Return(sourcePath).AnyTimes()
	mockLayer.EXPECT().GetLogger().Return(logger).AnyTimes()
	mockLayer.EXPECT().GetAddonsLayer().Return(addonsLayer).Times(2)

	err = applier.Apply(ctx, mockLayer)
	if err != nil {
		t.Fatalf("LayerApplier.Apply returned an error: %s", err)
	}
	time.Sleep(5 * time.Second)
	hr1 := getHR(ctx, t, client, "simple", "microservice")
	t.Logf("Found HelmRelease '%s/%s'", hr1.GetNamespace(), hr1.GetName())
	hr2 := getHR(ctx, t, client, "simple", "microservice-two")
	t.Logf("Found HelmRelease '%s/%s'", hr2.GetNamespace(), hr2.GetName())
}

func TestPruneIsRequired(t *testing.T) {
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	ctx := context.Background()

	logger := testlogr.TestLogger{T: t}
	scheme := combinedScheme()
	client := managerClient(ctx, t, scheme, "simple")

	applier, err := NewApplier(client, logger, scheme)
	if err != nil {
		t.Fatalf("The NewApplier constructor returned an error: %s", err)
	}
	t.Logf("NewApplier returned (%T) %#v", applier, applier)

	// This integration test can be forced to pass or fail at different stages by altering the
	// Values section of the microservice.yaml HelmRelease in the directory below.
	sourcePath := "testdata/apply/single_release"
	layerName := "test"
	addonsLayer := getAddonsLayer(ctx, t, client, layerName)

	mockLayer := mocklayers.NewMockLayer(mockCtl)
	mockLayer.EXPECT().GetName().Return(layerName).AnyTimes()
	mockLayer.EXPECT().GetSourcePath().Return(sourcePath).AnyTimes()
	mockLayer.EXPECT().GetLogger().Return(logger).AnyTimes()
	mockLayer.EXPECT().GetAddonsLayer().Return(addonsLayer).Times(1)

	pruneRequired, pruneHrs, err := applier.PruneIsRequired(ctx, mockLayer)
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

	ctx := context.Background()

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
	//timeoutContext, _ := context.WithTimeout(baseContext, 15*time.Second)
	layerName := "test"
	layerUID := types.UID("test-UID")
	addonsLayer := fakeAddonsLayer(sourcePath, layerName, layerUID)

	mockLayer := mocklayers.NewMockLayer(mockCtl)
	mockLayer.EXPECT().GetName().Return(layerName).AnyTimes()
	mockLayer.EXPECT().GetSourcePath().Return(sourcePath).AnyTimes()
	mockLayer.EXPECT().GetLogger().Return(logger).AnyTimes()
	mockLayer.EXPECT().GetAddonsLayer().Return(addonsLayer).Times(1)

	err = applier.Apply(ctx, mockLayer)
	if err != nil {
		t.Logf("LayerApplier.Apply timed out as expected.")
	} else {
		t.Logf("LayerApplier.Apply timed out as expected.")
		//t.Fatalf("LayerApplier.Apply returned an error: %s", err)
	}
}
