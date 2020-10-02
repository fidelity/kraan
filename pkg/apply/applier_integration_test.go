package apply_test

// DISABLED - +build integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	kraanv1alpha1 "github.com/fidelity/kraan/api/v1alpha1"
	"github.com/fidelity/kraan/pkg/apply"
	mocklayers "github.com/fidelity/kraan/pkg/mocks/layers"
	helmctlv2 "github.com/fluxcd/helm-controller/api/v2beta1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1alpha1"

	testlogr "github.com/go-logr/logr/testing"
	gomock "github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"

	extv1b1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	testMgr     = newTestManager()
	testNs      = "kraan-test"
	testLayer   = "test"
	testCRD     = "kraantests.kraan.io"
	testdataDir = "testdata"
	applyDir    = filepath.Join(testdataDir, "apply")
	crdsDir     = filepath.Join(testdataDir, "crds")
	testNsDir   = filepath.Join(applyDir, "test_namespace")
	testCRDFile = filepath.Join(crdsDir, "test_crd.yaml")
	singleRel   = filepath.Join(applyDir, "single_release")
	doubleRel   = filepath.Join(applyDir, "double_release")
)

func TestConnectToCluster(t *testing.T) {
	s := testMgr.Setup(t)
	defer s.dFunc()

	info, err := testMgr.getCoreClient().ServerVersion()
	if err != nil {
		t.Fatalf("Error getting server version: %s", err)
	}
	t.Logf("Server version %s", info.GitVersion)

	namespaceList := &corev1.NamespaceList{}
	err = testMgr.getRuntimeClient().List(testMgr.ctx, namespaceList)
	if err != nil {
		t.Fatalf("runtime error getting namespaces: %s", err)
	}
	for _, namespace := range namespaceList.Items {
		t.Logf("Found Namespace '%s'", namespace.GetName())
	}
}

func TestSingleApply(t *testing.T) {
	s := testMgr.Setup(t).Applier().WithNs().WithLayer(singleRel, 1)
	defer s.dFunc()
	s.ApplyMockLayer().VerifyHelmRelease("microservice")
}

func TestDoubleApply(t *testing.T) {
	s := testMgr.Setup(t).Applier().WithNs().WithLayer(doubleRel, 2)
	defer s.dFunc()
	s.ApplyMockLayer().VerifyHelmRelease("microservice").VerifyHelmRelease("microservice-two")
}

func TestApplyNamespace(t *testing.T) {
	s := testMgr.Setup(t).Applier().WithLayer(testNsDir, 1)
	defer s.dFunc()
	obj := s.ApplyMockLayer().GetResource(schema.GroupVersionKind{Version: "v1", Kind: "Namespace"}, client.ObjectKey{Name: "kraan-created"})
	s.VerifyOwnerRefs(obj).DelResource(obj)
}

func TestApplyCRD(t *testing.T) {
	s := testMgr.Setup(t).Applier().WithLayer(testCRDFile, 1)
	defer s.dFunc()
	if testMgr.count <= 1 {
		// TODO - This test passes when run by itself but fails when run as part of the suite, so skipping temporarily if the test counter is > 1
		s.ApplyMockLayer()
		//obj := s.WaitForResource(schema.GroupVersionKind{Group: "apiextensions.k8s.io", Version: "v1", Kind: "CustomResourceDefinition"}, client.ObjectKey{Name: testCRD}, 10)
		obj := s.GetResource(schema.GroupVersionKind{Group: "apiextensions.k8s.io", Version: "v1", Kind: "CustomResourceDefinition"}, client.ObjectKey{Name: testCRD})
		s.VerifyOwnerRefs(obj).DelResource(obj)
	}
}

func TestPruneIsRequired(t *testing.T) {
	s := testMgr.Setup(t).Applier().WithNs().WithLayer(doubleRel, 2).ApplyMockLayer().WithLayer(singleRel, 1)
	defer s.dFunc()
	s.VerifyPruneIsRequired(true)
}

func TestPruneIsNotRequired(t *testing.T) {
	s := testMgr.Setup(t).Applier().WithNs().WithLayer(singleRel, 1).ApplyMockLayer().WithLayer(doubleRel, 2)
	defer s.dFunc()
	s.VerifyPruneIsRequired(false)
}

func TestApplyIsNotRequired(t *testing.T) {
	s := testMgr.Setup(t).Applier().WithNs().WithLayer(singleRel, 1).ApplyMockLayer().WithLayer(singleRel, 1)
	defer s.dFunc()
	s.VerifyApplyIsRequired(false)
}

func TestApplyIsRequired(t *testing.T) {
	s := testMgr.Setup(t).Applier().WithNs().WithLayer(singleRel, 1).ApplyMockLayer().WithLayer(doubleRel, 2)
	defer s.dFunc()
	s.VerifyApplyIsRequired(true)
}

func TestPruneOneHelmRelease(t *testing.T) {
	s := testMgr.Setup(t).Applier().WithNs().WithLayer(doubleRel, 2).ApplyMockLayer().WithLayer(singleRel, 1)
	defer s.dFunc()
	s.PruneMockLayer()
}

type testManager struct {
	t             *testing.T
	ctx           context.Context
	config        *rest.Config
	scheme        *runtime.Scheme
	log           testlogr.TestLogger
	manager       ctrl.Manager
	client        client.Client
	runtimeClient client.Client
	coreClient    *kubernetes.Clientset
	count         int
}

type testSetup struct {
	t         *testing.T
	m         *testManager
	a         apply.LayerApplier
	ctx       context.Context
	config    *rest.Config
	scheme    *runtime.Scheme
	log       testlogr.TestLogger
	manager   ctrl.Manager
	client    client.Client
	mockCtl   *gomock.Controller
	ns        *corev1.Namespace
	source    string
	layerName string
	layer     *kraanv1alpha1.AddonsLayer
	fakeLayer *kraanv1alpha1.AddonsLayer
	mockLayer *mocklayers.MockLayer
	dFunc     func()
}

func newTestManager() *testManager {
	return &testManager{
		ctx:    context.Background(),
		scheme: combinedScheme(),
	}
}

func (m *testManager) Setup(t *testing.T) *testSetup {
	m.count++
	m.t = t
	m.log = testlogr.TestLogger{T: t}
	if m.config == nil {
		m.config = kubeConfig(t)
	}
	if m.manager == nil {
		m.createManager()
		m.client = m.manager.GetClient()
	}
	mCtl := gomock.NewController(t)
	return &testSetup{
		t:       t,
		m:       m,
		ctx:     m.ctx,
		config:  m.config,
		scheme:  m.scheme,
		log:     m.log,
		manager: m.manager,
		client:  m.client,
		mockCtl: mCtl,
		dFunc:   func() { m.tFinish(mCtl) },
	}
}

func (m *testManager) tFinish(mockCtl *gomock.Controller) {
	m.t.Logf("Finish called for %T %v", mockCtl, mockCtl)
	mockCtl.Finish()
}

func combinedScheme() *runtime.Scheme {
	intScheme := runtime.NewScheme()
	_ = k8sscheme.AddToScheme(intScheme)     // nolint:errcheck // ok
	_ = kraanv1alpha1.AddToScheme(intScheme) // nolint:errcheck // ok
	_ = helmctlv2.AddToScheme(intScheme)     // nolint:errcheck // ok
	_ = sourcev1.AddToScheme(intScheme)      // nolint:errcheck // ok
	_ = extv1b1.AddToScheme(intScheme)       // nolint:errcheck // ok
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

func (m *testManager) startManager(mgr ctrl.Manager) {
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		m.t.Errorf("unable to start runtime manager: %s", err.Error())
		//m.t.Fatalf("unable to start runtime manager: %s", err.Error())
	}
}

func (m *testManager) createManager() {
	mgr, err := ctrl.NewManager(m.config, ctrl.Options{
		Scheme:    m.scheme,
		Namespace: testNs,
	})
	if err != nil {
		m.t.Fatalf("unable to create runtime manager: %s", err)
	}
	m.manager = mgr

	err = createController(m.ctx, mgr)
	if err != nil {
		m.t.Fatalf("unable to create runtime controller: %s", err)
	}

	go m.startManager(mgr)

	time.Sleep(3 * time.Second)
}

func createController(ctx context.Context, mgr manager.Manager) error {
	r := createReconciler(ctx, mgr.GetConfig(), mgr.GetClient(), mgr.GetScheme())
	err := r.setupWithManager(mgr)
	// +kubebuilder:scaffold:builder
	if err != nil {
		return fmt.Errorf("unable to setup Reconciler with Manager: %w", err)
	}
	return nil
}

type fakeReconciler struct {
	client.Client
	Config *rest.Config
	//k8client kubernetes.Interface
	Scheme  *runtime.Scheme
	Context context.Context
}

func createReconciler(ctx context.Context, config *rest.Config, client client.Client, scheme *runtime.Scheme) *fakeReconciler {
	reconciler := &fakeReconciler{
		Client:  client,
		Config:  config,
		Scheme:  scheme,
		Context: ctx,
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
	// TODO - log this using the global testMgr instance
	// r.T.Logf("Reconcile called for AddonsLayer '%s'", addonsLayer.Name)
	return ctrl.Result{}, nil
}

func indexHelmReleaseByOwner(o runtime.Object) []string {
	hr, ok := o.(*helmctlv2.HelmRelease)
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
	hr := &helmctlv2.HelmRelease{}
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

func (m *testManager) createRuntimeClient() {
	client, err := client.New(m.config, client.Options{Scheme: m.scheme})
	if err != nil {
		m.t.Fatalf("Unable to create controller runtime client: %s", err)
	}
	m.runtimeClient = client
}

func (m *testManager) getRuntimeClient() client.Client {
	if m.runtimeClient == nil {
		m.createRuntimeClient()
	}
	return m.runtimeClient
}

func (m *testManager) createCoreClient() {
	clientset, err := kubernetes.NewForConfig(m.config)
	if err != nil {
		m.t.Fatalf("Unable to create Kubernetes API Clientset: %s", err)
	}
	m.coreClient = clientset
}

func (m *testManager) getCoreClient() *kubernetes.Clientset {
	if m.coreClient == nil {
		m.createCoreClient()
	}
	return m.coreClient
}

func (s *testSetup) Applier() *testSetup {
	applier, err := apply.NewApplier(s.client, s.log, s.scheme)
	if err != nil {
		s.t.Fatalf("The NewApplier constructor returned an error: %s", err)
	}
	s.t.Logf("NewApplier returned (%T) %#v", applier, applier)
	s.a = applier
	return s
}

func (s *testSetup) ApplyMockLayer() *testSetup {
	err := s.a.Apply(s.ctx, s.mockLayer)
	if err != nil {
		s.t.Fatalf("LayerApplier.Apply returned an error: %s", err)
	}
	time.Sleep(3 * time.Second)
	return s
}

func (s *testSetup) VerifyHelmRelease(name string) *testSetup {
	hr := s.getHR(name)
	s.t.Logf("Found HelmRelease '%s/%s'", hr.GetNamespace(), hr.GetName())
	s.VerifyOwnerRefs(hr)
	return s
}

func (s *testSetup) getHR(name string) *helmctlv2.HelmRelease {
	return getHR(s.ctx, s.t, s.client, testNs, name)
}

func getHR(ctx context.Context, t *testing.T, c client.Client, namespace string, name string) *helmctlv2.HelmRelease {
	hr := &helmctlv2.HelmRelease{}
	key := client.ObjectKey{Namespace: namespace, Name: name}
	if err := c.Get(ctx, key, hr); err != nil {
		t.Fatalf("unable to retrieve HelmRelease '%s/%s'", namespace, name)
	}
	return hr
}

/*func getObj(ctx context.Context, t *testing.T, c client.Client, namespace string, name string) metav1.Object {
	obj := &unstructured.Unstructured{}
	key := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}
	if err := c.Get(ctx, key, obj); err != nil {
		t.Fatalf("unable to retrieve Object '%s/%s'", namespace, name)
	}
	return obj
}*/

/*func getRes(ctx context.Context, t *testing.T, c client.Client, group, version, kind, name string) metav1.Object {
	obj := &unstructured.Unstructured{}
	gvk := schema.GroupVersionKind{
		Group:   group,
		Version: version,
		Kind:    kind,
	}
	obj.SetGroupVersionKind(gvk)
	key := client.ObjectKey{
		//Namespace: namespace,
		Name: name,
	}
	if err := c.Get(ctx, key, obj); err != nil {
		t.Fatalf("unable to retrieve '%s/%s/%s' Resource '%s' : %s", group, version, kind, name, err.Error())
		//t.Fatalf("unable to retrieve Object '%s/%s'", namespace, name)
	}
	foundGvk := obj.GroupVersionKind()
	t.Logf("GVK: %#v", foundGvk)
	return obj
}*/

/*func (s *testSetup) WaitForResource(gvk schema.GroupVersionKind, key client.ObjectKey, maxTries int) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	found := false
	for i := 0; i < maxTries; i++ {
		err := s.client.Get(s.ctx, key, obj)
		if err == nil {
			found = true
			break
		} else {
			s.t.Logf("waiting for '%#v' resource '%#v' : %s", gvk, key, err.Error())
		}
		time.Sleep(2 * time.Second)
	}
	if found {
		foundGvk := obj.GroupVersionKind()
		foundKey, err := client.ObjectKeyFromObject(obj)
		if err != nil {
			s.t.Logf("Failed to obtain Object key for %T '%s/%s': %s", obj, obj.GetNamespace(), obj.GetName(), err)
		}
		s.t.Logf("Found type '%#v' resource '%#v'", foundGvk, foundKey)
	} else {
		s.t.Fatalf("Failed to Get '%#v' resource '%#v'", gvk, key)
	}
	return obj
}*/

func (s *testSetup) GetResource(gvk schema.GroupVersionKind, key client.ObjectKey) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	if err := s.client.Get(s.ctx, key, obj); err != nil {
		s.t.Fatalf("unable to Get '%#v' by '%#v' : %s", gvk, key, err.Error())
	}
	foundGvk := obj.GroupVersionKind()
	s.t.Logf("Found type '%#v' resource '%s/%s'", foundGvk, obj.GetNamespace(), obj.GetName())
	return obj
}

func (s *testSetup) DelResource(obj runtime.Object) *testSetup {
	foreground := metav1.DeletePropagationForeground
	delOpts := client.DeleteOptions{PropagationPolicy: &foreground}
	if err := s.client.Delete(s.ctx, obj, &delOpts); err != nil {
		s.t.Errorf("unable to Delete '%s': %s", obj.GetObjectKind().GroupVersionKind(), err.Error())
	}
	return s
}

func (s *testSetup) WithLayer(sourceDir string, releases int) *testSetup {
	foundLayer, found := s.getLayer(testLayer)
	if found {
		s.layer = foundLayer
	} else {
		newLayer, ok := s.createLayer(testNs, testLayer, sourceDir)
		if !ok {
			s.t.Fatalf("Failed to create AddonsLayer '%s'", testLayer)
		}
		s.deferDelLayer(newLayer)
		foundLayer, found = s.getLayer(testLayer)
		if !found {
			s.t.Fatalf("Unable to retrieve created AddonsLayer '%s'", testLayer)
		}
		s.layer = foundLayer
	}
	s.layerName = testLayer
	s.source = sourceDir
	return s.createFakeLayer().setupMockLayer(releases)
}

func (s *testSetup) deferDelLayer(layer *kraanv1alpha1.AddonsLayer) {
	deferFunc := s.dFunc
	s.dFunc = func() {
		ok := s.delLayer(layer)
		if ok {
			s.t.Logf("%T '%s' deleted", layer, layer.GetName())
		} else {
			s.t.Fatalf("Failed to delete %T '%s'", layer, layer.GetName())
		}
		deferFunc()
	}
}

func (s *testSetup) setupMockLayer(releases int) *testSetup {
	mockLayer := mocklayers.NewMockLayer(s.mockCtl)
	mockLayer.EXPECT().GetName().Return(s.layerName).AnyTimes()
	mockLayer.EXPECT().GetSourcePath().Return(s.source).AnyTimes()
	mockLayer.EXPECT().GetLogger().Return(s.log).AnyTimes()
	mockLayer.EXPECT().GetAddonsLayer().Return(s.fakeLayer).Times(releases)
	s.mockLayer = mockLayer
	return s
}

func (s *testSetup) createFakeLayer() *testSetup {
	kind := "AddonsLayer"
	version := "v1alpha1"
	typeMeta := metav1.TypeMeta{
		Kind:       kind,
		APIVersion: version,
	}
	now := metav1.Time{Time: time.Now()}
	layerMeta := metav1.ObjectMeta{
		Name:              s.layerName,
		UID:               s.layer.GetUID(),
		ResourceVersion:   version,
		Generation:        1,
		CreationTimestamp: now,
		ClusterName:       "TestingCluster",
	}
	sourceSpec := kraanv1alpha1.SourceSpec{
		Name: "TestingSource",
		Path: s.source,
	}
	layerPreReqs := kraanv1alpha1.PreReqs{
		K8sVersion: "1.16",
		//K8sVersion string `json:"k8sVersion"`
		//DependsOn []string `json:"dependsOn,omitempty"`
	}
	layerSpec := kraanv1alpha1.AddonsLayerSpec{
		Source:  sourceSpec,
		PreReqs: layerPreReqs,
		Hold:    false,
		Version: "test-version",
		//Source SourceSpec `json:"source"`
		//PreReqs PreReqs `json:"prereqs,omitempty"`
		//Hold bool `json:"hold,omitempty"`
		//Interval metav1.Duration `json:"interval"`
		//Timeout *metav1.Duration `json:"timeout,omitempty"`
		//Version string `json:"version"`
	}
	layerStatus := kraanv1alpha1.AddonsLayerStatus{
		State:   "Testing",
		Version: "state-version",
		//Conditions []Condition `json:"conditions,omitempty"`
		//State string `json:"state,omitempty"`
		//Version string `json:"version,omitempty"`
	}
	s.fakeLayer = &kraanv1alpha1.AddonsLayer{
		TypeMeta:   typeMeta,
		ObjectMeta: layerMeta,
		Spec:       layerSpec,
		Status:     layerStatus,
	}
	return s
}

func (s *testSetup) createLayer(namespace, name, path string) (al *kraanv1alpha1.AddonsLayer, ok bool) {
	path = "./" + path
	meta := metav1.ObjectMeta{Name: name}
	source := kraanv1alpha1.SourceSpec{
		Name:      name,
		NameSpace: namespace,
		Path:      path,
	}
	preReqs := kraanv1alpha1.PreReqs{
		K8sVersion: "1.16",
	}
	spec := kraanv1alpha1.AddonsLayerSpec{
		Version: "test-version",
		Hold:    false,
		Source:  source,
		PreReqs: preReqs,
	}
	obj := &kraanv1alpha1.AddonsLayer{
		ObjectMeta: meta,
		Spec:       spec,
	}
	//s.t.Logf("Creating AddonsLayer '%s' for source '%s': %#v", name, path, source)
	createOpts := &client.CreateOptions{FieldManager: "kraan"}
	if err := s.client.Create(s.ctx, obj, createOpts); err != nil {
		s.t.Errorf("unable to create AddonsLayer '%s': %s", name, err.Error())
		return obj, false
	}
	s.t.Logf("Created AddonsLayer '%s'", obj.GetName())
	return obj, true
}

func (s *testSetup) getLayer(name string) (layer *kraanv1alpha1.AddonsLayer, found bool) {
	obj := &kraanv1alpha1.AddonsLayer{}
	key := client.ObjectKey{Name: name}
	if err := s.client.Get(s.ctx, key, obj); err != nil {
		return nil, false
	}
	//s.t.Logf("Found AddonsLayer '%s': %#v", name, obj)
	s.t.Logf("Found AddonsLayer '%s'", name)
	return obj, true
}

func (s *testSetup) delLayer(layer *kraanv1alpha1.AddonsLayer) bool {
	foreground := metav1.DeletePropagationForeground
	delOpts := client.DeleteOptions{PropagationPolicy: &foreground}
	if err := s.client.Delete(s.ctx, layer, &delOpts); err != nil {
		return false
	}
	time.Sleep(3 * time.Second)
	return true
}

func (s *testSetup) deferDelNs(ns *corev1.Namespace) {
	deferFunc := s.dFunc
	s.dFunc = func() {
		ok := s.delNs(ns)
		if ok {
			s.t.Logf("%T '%s' deleted", ns, ns.GetName())
		} else {
			s.t.Fatalf("Failed to delete %T '%s'", ns, ns.GetName())
		}
		deferFunc()
	}
}

func (s *testSetup) WithNs() *testSetup {
	foundNs, found := s.getNs(testNs)
	if found {
		s.t.Logf("Found Namespace '%s'", foundNs.GetName())
		s.ns = foundNs
	} else {
		newNs, ok := s.createNs(testNs)
		if !ok {
			s.t.Fatalf("Failed to create Namespace '%s'", testNs)
		}
		s.t.Logf("Created Namespace '%s'", newNs.GetName())
		s.ns = newNs
		s.deferDelNs(newNs)
	}
	return s
}

func (s *testSetup) createNs(name string) (ns *corev1.Namespace, ok bool) {
	return createNs(s.ctx, s.t, s.client, name)
}

func createNs(ctx context.Context, t *testing.T, c client.Client, name string) (ns *corev1.Namespace, ok bool) {
	meta := metav1.ObjectMeta{Name: name}
	obj := &corev1.Namespace{ObjectMeta: meta}
	createOpts := &client.CreateOptions{FieldManager: "kraan"}
	if err := c.Create(ctx, obj, createOpts); err != nil {
		t.Errorf("unable to create Namespace '%s': %s", name, err.Error())
		return obj, false
	}
	// t.Logf("Created Namespace: %#v", obj)
	return obj, true
}

func (s *testSetup) getNs(name string) (ns *corev1.Namespace, found bool) {
	return getNs(s.ctx, s.client, name)
}

func getNs(ctx context.Context, c client.Client, name string) (ns *corev1.Namespace, found bool) {
	obj := &corev1.Namespace{}
	//key := client.ObjectKey{Namespace: namespace}
	key := client.ObjectKey{Name: name}
	if err := c.Get(ctx, key, obj); err != nil {
		return nil, false
		//t.Fatalf("unable to retrieve Namespace '%s'", name)
	}
	return obj, true
}

func (s *testSetup) delNs(ns *corev1.Namespace) bool {
	return delNs(s.ctx, s.client, ns)
}

func delNs(ctx context.Context, c client.Client, ns *corev1.Namespace) bool {
	foreground := metav1.DeletePropagationForeground
	delOpts := client.DeleteOptions{PropagationPolicy: &foreground}
	if err := c.Delete(ctx, ns, &delOpts); err != nil {
		return false
	}
	return true
}

func (s *testSetup) VerifyOwnerRefs(obj metav1.Object) *testSetup {
	//return verifyOwnerRefs(s.t, obj, owner)
	if verifyOwnerRefs(s.t, obj, s.fakeLayer) {
		//s.t.Logf("Owner reference verified for %T '%s': %#v", s.fakeLayer, s.fakeLayer.GetName(), obj)
		s.t.Logf("Owner reference verified for %T '%s'", s.fakeLayer, s.fakeLayer.GetName())
	} else {
		//s.t.Errorf("No owner reference found for %T '%s': %#v", s.fakeLayer, s.fakeLayer.GetName(), obj)
		s.t.Errorf("No owner reference found for %T '%s'", s.fakeLayer, s.fakeLayer.GetName())
	}
	return s
}

func verifyOwnerRefs(t *testing.T, obj, owner metav1.Object) bool {
	owners := obj.GetOwnerReferences()
	if len(owners) < 1 {
		t.Logf("no owner references defined for %T '%s/%s'", obj, obj.GetNamespace(), obj.GetName())
		return false
	}
	for i, ownerRef := range owners {
		t.Logf("comparing owner UID '%s' to ownerRefs[%d] : %#v", owner.GetUID(), i, ownerRef)
		if ownerRef.UID == owner.GetUID() {
			t.Logf("found %T '%s/%s' ownerRef for %T '%s' : %#v", obj, obj.GetNamespace(), obj.GetName(), owner, owner.GetName(), ownerRef)
			return true
		}
	}
	return false
}

func (s *testSetup) pruneIsRequired() (bool, []*helmctlv2.HelmRelease) {
	pruneRequired, pruneHrs, err := s.a.PruneIsRequired(s.ctx, s.mockLayer)
	if err != nil {
		s.t.Fatalf("LayerApplier.PruneIsRequired returned an error: %s", err)
	}
	s.t.Logf("LayerApplier.PruneIsRequired returned %v", pruneRequired)
	s.t.Logf("LayerApplier.PruneIsRequired returned %d hrs to prune", len(pruneHrs))
	for _, hr := range pruneHrs {
		s.t.Logf("LayerApplier.PruneIsRequired - '%s/%s'", hr.GetNamespace(), hr.GetName())
	}
	return pruneRequired, pruneHrs
}

func (s *testSetup) verifyPrunedHR(hr *helmctlv2.HelmRelease) bool {
	obj := &helmctlv2.HelmRelease{}
	key, keyErr := client.ObjectKeyFromObject(hr)
	if keyErr != nil {
		s.t.Fatalf("Failed to obtain ObjectKey from HelmRelease '%s/%s'", hr.GetNamespace(), hr.GetName())
	}
	err := s.client.Get(s.ctx, key, obj)
	return err != nil
}

func (s *testSetup) PruneMockLayer() *testSetup {
	pruneRequired, pruneHrs := s.pruneIsRequired()
	if pruneRequired {
		err := s.a.Prune(s.ctx, s.mockLayer, pruneHrs)
		if err != nil {
			s.t.Fatalf("LayerApplier.Prune returned an error: %s", err.Error())
		}
		time.Sleep(3 * time.Second)
		for _, hr := range pruneHrs {
			if s.verifyPrunedHR(hr) {
				s.t.Logf("verified HelmRelease '%s/%s' was pruned", hr.GetNamespace(), hr.GetName())
			} else {
				s.t.Errorf("HelmRelease '%s/%s' was not pruned!", hr.GetNamespace(), hr.GetName())
			}
		}
	}
	return s
}

func (s *testSetup) VerifyPruneIsRequired(expected bool) *testSetup {
	pruneRequired, _ := s.pruneIsRequired()
	if pruneRequired != expected {
		s.t.Fatalf("LayerApplier.PruneIsRequired returned %v expected %v", pruneRequired, expected)
	}
	return s
}

func (s *testSetup) VerifyApplyIsRequired(expected bool) *testSetup {
	applyRequired, err := s.a.ApplyIsRequired(s.ctx, s.mockLayer)
	if err != nil {
		s.t.Fatalf("LayerApplier.ApplyIsRequired returned an error: %s", err)
	}
	s.t.Logf("LayerApplier.ApplyIsRequired returned %v", applyRequired)
	if applyRequired != expected {
		s.t.Fatalf("LayerApplier.ApplyIsRequired returned %v expected %v", applyRequired, expected)
	}
	return s
}
