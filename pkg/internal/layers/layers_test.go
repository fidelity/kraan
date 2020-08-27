package layers // nolint:package // unit tests should be in same package as code under test

/*

The mockgen tool generates the MockExecProvider type defined in the kubectl/mockExecProvider.go code file.

From the project root directory, you can generate mock definitions for interfaces in individual code files by calling mockgen.  Example:
	mockgen -destination=pkg/internal/kubectl/mockExecProvider.go -package=kubectl -source=pkg/internal/kubectl/execProvider.go \
	gitlab.fmr.com/common-platform/addons-manager/pkg/internal/kubectl ExecProvider

Or you can generate all the

Add a go:generate annotation above the package statement in all the code files containing interfaces that you want to mock.  Example:
//go:generate mockgen -destination=mockExecProvider.go -package=kubectl -source=execProvider.go . ExecProvider
//go:generate mockgen -destination=../mocks/logr/mockLogger.go -package=mocks github.com/go-logr/logr Logger

From the project root directory, you can then generate mocks for all the interfaces that have a go:generate annotation by running 'go generate ./...'.

*/
import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"

	kraanv1alpha1 "github.com/fidelity/kraan/pkg/api/v1alpha1"
	"github.com/go-logr/logr"
	testlogr "github.com/go-logr/logr/testing"
	"github.com/paulcarlton-ww/go-utils/pkg/goutils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakeK8s "k8s.io/client-go/kubernetes/fake"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	testScheme = runtime.NewScheme()
	// testCtx    = context.Background()
)

func init() {
	_ = k8sscheme.AddToScheme(testScheme)     // nolint:errcheck // ok
	_ = kraanv1alpha1.AddToScheme(testScheme) // nolint:errcheck // ok
}

func fakeLogger() logr.Logger {
	return testlogr.NullLogger{}
}

func TestCreateLayer(t *testing.T) {

}

/*
func getCrdFromFile(fileName string) (*apiextypes.CustomResourceDefinition, error) {
	buffer, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	crd := &apiextypes.CustomResourceDefinition{}
	err = yaml.Unmarshal(buffer, crd)
	if err != nil {
		return nil, err
	}
	return crd, nil
}
*/
/*
func getLayerFromFile(fileName string) (*kraanv1alpha1.AddonsLayer, error) {
	buffer, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	addonsLayer := &kraanv1alpha1.AddonsLayer{}
	err = json.Unmarshal(buffer, addonsLayer)
	if err != nil {
		return nil, err
	}
	return addonsLayer, nil
}
*/
func getLayersFromFile(fileName string) (*kraanv1alpha1.AddonsLayerList, error) {
	buffer, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	addonsLayers := &kraanv1alpha1.AddonsLayerList{}
	err = json.Unmarshal(buffer, addonsLayers)
	if err != nil {
		return nil, err
	}
	return addonsLayers, nil
}

func getFromList(name string, layerList *kraanv1alpha1.AddonsLayerList) *kraanv1alpha1.AddonsLayer {
	for _, item := range layerList.Items {
		if item.ObjectMeta.Name == name {
			return &item
		}
	}
	return nil
}

func getLayer(layerName, testDataFileName string) (Layer, error) {
	logger := fakeLogger()
	layers, err := getLayersFromFile(testDataFileName)
	if err != nil {
		return nil, err
	}
	/*
		crd, err := getCrdFromFile("../../../config/crd/bases/kraan.io_addonslayers.yaml") // nolint:
		if err != nil {
			return nil, err
		}
	*/
	client := fake.NewFakeClientWithScheme(testScheme, layers)
	// unable to get pass layers and crd definition to fake k8sclient, but not a blocker at this stage
	fakeK8sClient := fakeK8s.NewSimpleClientset()
	data := getFromList(layerName, layers)
	if data == nil {
		return nil, fmt.Errorf("failed to find item: %s in test data", layerName)
	}
	return CreateLayer(context.Background(), client, fakeK8sClient, logger, data), nil
}

func testRequeue(t *testing.T, l Layer) bool {
	l.SetRequeue()
	k, ok := l.(*KraanLayer)
	if !ok {
		t.Errorf("failed to cast layer interface to *KraanLayer")
		return false
	}
	if !k.requeue {
		t.Errorf("failed to set requeue using SetRequeue")
		return false
	}
	if !l.NeedsRequeue() {
		t.Errorf("failed to set requeue using SetRequeue")
		return false
	}
	return true
}

func TestSetRequeue(t *testing.T) {
	l, e := getLayer("bootstrap", "testdata/layerslist1.json")
	if e != nil {
		t.Fatalf("failed to create layer, error: %s", e.Error())
	}
	if !testRequeue(t, l) {
		return
	}
	if !testRequeue(t, l) { // Verify it works if set again when already set
		return
	}
}

func compareConditions(actual, expected []kraanv1alpha1.Condition) bool {
	if len(actual) != len(expected) {
		return false
	}
	for index, condition := range actual {
		expect := expected[index]
		if condition.Message != expect.Message ||
			condition.Reason != expect.Reason ||
			condition.Status != expect.Status ||
			condition.Type != expect.Type ||
			condition.Version != expect.Version {
			return false
		}
	}
	return true
}

func compareStatus(t *testing.T, actual, expected *kraanv1alpha1.AddonsLayerStatus) {
	if actual.State != expected.State ||
		actual.Version != expected.Version ||
		!compareConditions(actual.Conditions, expected.Conditions) {
		actualJSON, err := goutils.ToJSON(actual)
		if err != nil {
			t.Fatalf("failed to generate json output for actual result, error: %s", err.Error())
		}

		expectedJSON, err := goutils.ToJSON(expected)
		if err != nil {
			t.Fatalf("failed to generate json output for expected result, error: %s", err.Error())
		}
		t.Fatalf("status not set to expected values...\n\nActual...\n%s\n\nExpected...\n%s",
			actualJSON, expectedJSON)
	}
}

func TestSetStatusSetting(t *testing.T) {
	type testsData struct {
		setFunc  func()
		expected *kraanv1alpha1.AddonsLayerStatus
	}

	l, e := getLayer("empty-status", "testdata/layersdata.json")
	if e != nil {
		t.Fatalf("failed to create layer, error: %s", e.Error())
	}

	tests := []testsData{{
		setFunc: l.SetStatusK8sVersion,
		expected: &kraanv1alpha1.AddonsLayerStatus{
			State:   kraanv1alpha1.K8sVersionCondition,
			Version: "1.16-fideks-0.0.79",
			Conditions: []kraanv1alpha1.Condition{{
				Status:  corev1.ConditionTrue,
				Version: "1.16-fideks-0.0.79",
				Type:    kraanv1alpha1.K8sVersionCondition,
				Reason:  kraanv1alpha1.AddonsLayerK8sVersionReason,
				Message: kraanv1alpha1.AddonsLayerK8sVersionMsg},
			},
		}},
		{
			setFunc: l.SetStatusPruning,
			expected: &kraanv1alpha1.AddonsLayerStatus{
				State:   kraanv1alpha1.PruningCondition,
				Version: "1.16-fideks-0.0.79",
				Conditions: []kraanv1alpha1.Condition{{
					Status:  corev1.ConditionTrue,
					Version: "1.16-fideks-0.0.79",
					Type:    kraanv1alpha1.PruningCondition,
					Reason:  kraanv1alpha1.AddonsLayerPruningReason,
					Message: kraanv1alpha1.AddonsLayerPruningMsg},
				},
			}},
	}

	for _, test := range tests {
		test.setFunc()
		compareStatus(t, l.GetFullStatus(), test.expected)
	}
}
