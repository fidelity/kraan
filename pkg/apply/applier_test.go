package apply_test

import (
	"encoding/json"
	"io/ioutil"
	"testing"
	"time"

	helmctlv2 "github.com/fluxcd/helm-controller/api/v2beta1"
	"github.com/go-logr/logr"
	testlogr "github.com/go-logr/logr/testing"
	gomock "github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	types "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kraanv1alpha1 "github.com/fidelity/kraan/api/v1alpha1"
	"github.com/fidelity/kraan/pkg/apply"
	"github.com/fidelity/kraan/pkg/internal/kubectl"
	kubectlmocks "github.com/fidelity/kraan/pkg/internal/mocks/kubectl"
)

const (
	addonsFileName       = "addons.json"
	helmrReleasesFileName = "helmreleases.json"
)

var (
	testScheme = runtime.NewScheme()
)

func init() {
	_ = corev1.AddToScheme(testScheme)        // nolint:errcheck // ok
	_ = kraanv1alpha1.AddToScheme(testScheme) // nolint:errcheck // ok
	_ = helmctlv2.AddToScheme(testScheme)     // nolint:errcheck // ok
}

func getAddonsFromFile(t *testing.T, fileName string) *kraanv1alpha1.AddonsLayerList {
	buffer, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Fatalf("failed to read AddonLayersList file: %s, %s", fileName, err)

		return nil
	}

	addons := &kraanv1alpha1.AddonsLayerList{}

	err = json.Unmarshal(buffer, addons)
	if err != nil {
		t.Fatalf("failed to unmarshall AddonLayersList file: %s, %s", fileName, err)

		return nil
	}
	return addons
}

func getHelmReleasesFromFile(t *testing.T, fileName string) *helmctlv2.HelmReleaseList {
	buffer, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Fatalf("failed to read HelmReleaseList file: %s, %s", fileName, err)

		return nil
	}

	helmReleases := &helmctlv2.HelmReleaseList{}

	err = json.Unmarshal(buffer, helmReleases)
	if err != nil {
		t.Fatalf("failed to unmarshall HelmReleaseList file: %s, %s", fileName, err)

		return nil
	}
	return helmReleases
}

func getApplierParams(t *testing.T, execpassName,
	addonLayersFileName, helmReleasesFileName string,
	client client.Client, scheme *runtime.Scheme) []interface{} {
	addonsList := getAddonsFromFile(t, addonsFileName)

	if t.Failed() {
		return nil
	}

	helmReleasesList := getHelmReleasesFromFile(t, helmReleasesFileName)

	if t.Failed() {
		return nil
	}

	if client == nil {
		client = fake.NewFakeClientWithScheme(scheme,
			addonsList, helmReleasesList /*, gitReposList, helmReposList*/)
	}

	return []interface{}{
		client,
		logr.Discard(),
		scheme,
	}
}

func createApplier(t *testing.T, params []interface{}) apply.LayerApplier {
	applier, err := apply.NewApplier(
		params[0].(client.Client),
		params[1].(logr.Logger),
		params[2].(*runtime.Scheme))
	if err != nil {
		t.Fatalf("failed to create applier, %s", err)
	}
	return applier
}

func fakeAddonsLayer(sourcePath, layerName string, layerUID types.UID) *kraanv1alpha1.AddonsLayer { //nolint
	kind := kraanv1alpha1.AddonsLayerKind
	version := kraanv1alpha1.GroupVersion.Version
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
	sourceSpec := kraanv1alpha1.SourceSpec{
		Name: "TestingSource",
		Path: sourcePath,
	}
	layerPreReqs := kraanv1alpha1.PreReqs{
		K8sVersion: "1.15.3",
	}
	layerSpec := kraanv1alpha1.AddonsLayerSpec{
		Source:  sourceSpec,
		PreReqs: layerPreReqs,
		Hold:    false,
		Version: "0.0.1",
	}
	layerStatus := kraanv1alpha1.AddonsLayerStatus{
		State:   "Testing",
		Version: "0.0.1",
	}
	addonsLayer := &kraanv1alpha1.AddonsLayer{
		TypeMeta:   typeMeta,
		ObjectMeta: layerMeta,
		Spec:       layerSpec,
		Status:     layerStatus,
	}
	return addonsLayer
}

func TestNewApplier(t *testing.T) {
	logger := testlogr.TestLogger{T: t}
	client := fake.NewFakeClientWithScheme(testScheme)
	applier, err := apply.NewApplier(client, logger, testScheme)
	if err != nil {
		t.Fatalf("The NewApplier constructor returned an error: %s", err)
	}
	t.Logf("NewApplier returned (%T) %#v", applier, applier)
}

func TestNewApplierWithMockKubectl(t *testing.T) {
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	mockKubectl := kubectlmocks.NewMockKubectl(mockCtl)
	newKFunc := func(logger logr.Logger) (kubectl.Kubectl, error) {
		return mockKubectl, nil
	}
	apply.SetNewKubectlFunc(newKFunc)

	logger := testlogr.TestLogger{T: t}
	client := fake.NewFakeClientWithScheme(testScheme)
	applier, err := apply.NewApplier(client, logger, testScheme)
	if err != nil {
		t.Fatalf("The NewApplier constructor returned an error: %s", err)
	}
	t.Logf("NewApplier returned (%T) %#v", applier, applier)
}
