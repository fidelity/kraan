package apply_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	helmctlv2 "github.com/fluxcd/helm-controller/api/v2beta1"
	"github.com/go-logr/logr"
	testlogr "github.com/go-logr/logr/testing"
	gomock "github.com/golang/mock/gomock"
	"github.com/paulcarlton-ww/goutils/pkg/testutils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	types "k8s.io/apimachinery/pkg/types"
	fakeK8s "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kraanv1alpha1 "github.com/fidelity/kraan/api/v1alpha1"
	"github.com/fidelity/kraan/pkg/apply"
	"github.com/fidelity/kraan/pkg/internal/kubectl"
	kubectlmocks "github.com/fidelity/kraan/pkg/internal/mocks/kubectl"
	"github.com/fidelity/kraan/pkg/layers"
)

const (
	addonsFileName              = "testdata/addons.json"
	helmReleasesFileName        = "testdata/helmreleases.json"
	orphan1HelmReleasesFileName = "testdata/orphaned1-helmreleases.json"
	orphan2HelmReleasesFileName = "testdata/orphaned2-helmreleases.json"
	orphan3HelmReleasesFileName = "testdata/orphaned3-helmreleases.json"
	bootstrapOrphaned           = "bootstrap/orphaned1"
	baseOrphaned                = "base/orphaned2"
	mgmtOrphaned                = "mgmt/orphaned3"
	appLayer                    = "apps"
	bootstrapLayer              = "bootstrap"
	k8sList                     = "List"
)

var (
	testScheme = runtime.NewScheme()
	fakeClient client.Client
)

func init() {
	_ = corev1.AddToScheme(testScheme)        // nolint:errcheck // ok
	_ = kraanv1alpha1.AddToScheme(testScheme) // nolint:errcheck // ok
	_ = helmctlv2.AddToScheme(testScheme)     // nolint:errcheck // ok
}

func getAddonsFromFiles(t *testing.T, fileNames ...string) *kraanv1alpha1.AddonsLayerList {
	addonsLayersList := &kraanv1alpha1.AddonsLayerList{
		TypeMeta: metav1.TypeMeta{
			Kind:       k8sList,
			APIVersion: fmt.Sprintf("%s/%s", kraanv1alpha1.GroupVersion.Version, kraanv1alpha1.GroupVersion.Version),
		},
		Items: make([]kraanv1alpha1.AddonsLayer, 0, 10),
	}

	for _, fileName := range fileNames {
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

		addonsLayersList.Items = append(addonsLayersList.Items, addons.Items...)
	}

	return addonsLayersList
}

func getAddonFromList(t *testing.T, name string, addonList *kraanv1alpha1.AddonsLayerList) *kraanv1alpha1.AddonsLayer {
	for _, item := range addonList.Items {
		if item.ObjectMeta.Name == name {
			return &item
		}
	}

	t.Fatalf("failed to find AddonsLayer: %s in list", name)

	return nil
}

func getLayer(t *testing.T, layerName, dataFileName string) layers.Layer { // nolint: unparam // ok
	fakeK8sClient := fakeK8s.NewSimpleClientset()
	data := getAddonFromList(t, layerName, getAddonsFromFiles(t, dataFileName))
	if data == nil {
		t.Fatalf("failed to read AddonsLayerList file: %s", dataFileName)
		return nil
	}
	fakeRecorder := record.NewFakeRecorder(1000)
	return layers.CreateLayer(context.Background(), fakeClient, fakeK8sClient, logr.Discard(), fakeRecorder, testScheme, data)
}

func getHelmReleasesFromFiles(t *testing.T, fileNames ...string) *helmctlv2.HelmReleaseList {
	helmReleasesList := &helmctlv2.HelmReleaseList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "List",
			APIVersion: fmt.Sprintf("%s/%s", helmctlv2.GroupVersion.Version, helmctlv2.GroupVersion.Version),
		},
		Items: make([]helmctlv2.HelmRelease, 0, 10),
	}

	for _, fileName := range fileNames {
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

		helmReleasesList.Items = append(helmReleasesList.Items, helmReleases.Items...)
	}
	return helmReleasesList
}

func getHelmReleaseFromList(t *testing.T, nameSpaceSlashName string, helmReleaseList *helmctlv2.HelmReleaseList) *helmctlv2.HelmRelease {
	for _, item := range helmReleaseList.Items {
		if fmt.Sprintf("%s/%s", item.Namespace, item.Name) == nameSpaceSlashName {
			return &item
		}
	}

	t.Fatalf("failed to find HelmRelease: %s in list", nameSpaceSlashName)

	return nil
}

func getApplierParams(t *testing.T, addonsFileNames, helmReleasesFileNames []string,
	client client.Client, scheme *runtime.Scheme) []interface{} { // nolint: unparam //ok
	addonsList := getAddonsFromFiles(t, addonsFileNames...)

	if t.Failed() {
		return nil
	}

	helmReleasesList := getHelmReleasesFromFiles(t, helmReleasesFileNames...)

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

func castToApplier(t *testing.T, i interface{}) apply.LayerApplier {
	if ep, ok := i.(apply.LayerApplier); ok {
		return ep
	}

	t.Fatalf("failed to cast interface to apply.LayerApplier")

	return nil
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

func TestGetOrphanedHelmReleases(t *testing.T) { // nolint: funlen //ok
	tests := []*testutils.DefTest{
		{
			Number:      1,
			Description: "one orphaned helm release, not this layer",
			Config: createApplier(t, getApplierParams(t,
				[]string{addonsFileName},
				[]string{helmReleasesFileName, orphan1HelmReleasesFileName},
				nil, testScheme)),
			Inputs: []interface{}{
				context.Background(),
				getLayer(t, appLayer, addonsFileName),
			},
			Expected: []interface{}{
				map[string]*helmctlv2.HelmRelease{
					bootstrapOrphaned: getHelmReleaseFromList(t, bootstrapOrphaned,
						getHelmReleasesFromFiles(t, orphan1HelmReleasesFileName)),
				},
				nil,
			},
			ResultsCompareFunc: testutils.CompareJSON,
			ResultsReportFunc:  testutils.ReportJSON,
		},
		{
			Number:      2,
			Description: "one orphaned helm release, but this layer",
			Config: createApplier(t, getApplierParams(t,
				[]string{addonsFileName},
				[]string{helmReleasesFileName, orphan1HelmReleasesFileName},
				nil, testScheme)),
			Inputs: []interface{}{
				context.Background(),
				getLayer(t, bootstrapLayer, addonsFileName),
			},
			Expected: []interface{}{
				map[string]*helmctlv2.HelmRelease{},
				nil,
			},
			ResultsCompareFunc: testutils.CompareJSON,
			ResultsReportFunc:  testutils.ReportJSON,
		},
		{
			Number:      3,
			Description: "no orphaned helm releases",
			Config: createApplier(t, getApplierParams(t,
				[]string{addonsFileName},
				[]string{helmReleasesFileName},
				nil, testScheme)),
			Inputs: []interface{}{
				context.Background(),
				getLayer(t, appLayer, addonsFileName),
			},
			Expected: []interface{}{
				map[string]*helmctlv2.HelmRelease{},
				nil,
			},
			ResultsCompareFunc: testutils.CompareJSON,
			ResultsReportFunc:  testutils.ReportJSON,
		},
		{
			Number:      4,
			Description: "multiple orphaned helm release, two not this layer",
			Config: createApplier(t, getApplierParams(t,
				[]string{addonsFileName},
				[]string{helmReleasesFileName,
					orphan1HelmReleasesFileName,
					orphan2HelmReleasesFileName,
					orphan3HelmReleasesFileName},
				nil, testScheme)),
			Inputs: []interface{}{
				context.Background(),
				getLayer(t, bootstrapLayer, addonsFileName),
			},
			Expected: []interface{}{
				map[string]*helmctlv2.HelmRelease{
					baseOrphaned: getHelmReleaseFromList(t, baseOrphaned,
						getHelmReleasesFromFiles(t, orphan2HelmReleasesFileName)),
					mgmtOrphaned: getHelmReleaseFromList(t, mgmtOrphaned,
						getHelmReleasesFromFiles(t, orphan3HelmReleasesFileName)),
				},
				nil,
			},
			ResultsCompareFunc: testutils.CompareJSON,
			ResultsReportFunc:  testutils.ReportJSON,
		},
	}

	testFunc := func(t *testing.T, testData *testutils.DefTest) bool {
		u := testutils.NewTestUtil(t, testData)

		u.CallPrepFunc()

		a := castToApplier(t, testData.Config)

		hrs, err := a.GetOrphanedHelmReleases(
			testData.Inputs[0].(context.Context),
			testData.Inputs[1].(layers.Layer))
		testData.Results = []interface{}{hrs, err}

		return u.CallCheckFunc()
	}

	for _, test := range tests {
		if !testFunc(t, test) {
			t.Fatalf("Test failed")

			return
		}
	}
}
