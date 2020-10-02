package layers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/go-logr/logr"
	testlogr "github.com/go-logr/logr/testing"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	fakeK8s "k8s.io/client-go/kubernetes/fake"
	fakeTest "k8s.io/client-go/testing"

	//k8sscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kraanv1alpha1 "github.com/fidelity/kraan/api/v1alpha1"
	"github.com/fidelity/kraan/pkg/internal/testutils"
	"github.com/fidelity/kraan/pkg/layers"
)

var (
	testScheme = runtime.NewScheme()
	// testCtx    = context.Background()
	fakeK8sClient *fakeK8s.Clientset
)

const (
	holdSet      = "hold-set"
	k8sPending   = "k8s-pending"
	emptyStatus  = "empty-status"
	noDepends    = "no-depends"
	oneDepends   = "one-depends"
	oneDependsV2 = "one-depends-v2"
	twoDepends   = "two-depends"
	k8sv16       = "k8s-v16"
	k8sv16_2     = "k8s-v16-2"
	//maxConditions = "max-conditions"
	layersData  = "testdata/layersdata.json"
	versionOne  = "0.1.01"
	layersData1 = "testdata/layersdata1.json"
	layersData2 = "testdata/layersdata2.json"
)

func init() {
	_ = corev1.AddToScheme(testScheme) // nolint:errcheck // ok
	//_ = k8sscheme.AddToScheme(testScheme)     // nolint:errcheck // ok
	_ = kraanv1alpha1.AddToScheme(testScheme) // nolint:errcheck // ok
}

func fakeLogger() logr.Logger {
	return testlogr.NullLogger{}
}

func TestCreateLayer(t *testing.T) {

}

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

func getLayer(layerName, testDataFileName string) (layers.Layer, error) {
	logger := fakeLogger()
	layerList, err := getLayersFromFile(testDataFileName)
	if err != nil {
		return nil, err
	}
	client := fake.NewFakeClientWithScheme(testScheme, layerList)
	fakeK8sClient = fakeK8s.NewSimpleClientset()
	data := getFromList(layerName, layerList)
	if data == nil {
		return nil, fmt.Errorf("failed to find item: %s in test data", layerName)
	}
	return layers.CreateLayer(context.Background(), client, fakeK8sClient, logger, data), nil
}

func testDelayedRequeue(t *testing.T, l layers.Layer) bool {
	l.SetDelayedRequeue()
	/*k, ok := l.(*layers.KraanLayer)
	if !ok {
		t.Errorf("failed to cast layer interface to *layers.KraanLayer")
		return false
	}
	if !k.requeue {
		t.Errorf("failed to set requeue using SetDelayedRequeue")
		return false
	}*/
	if !l.NeedsRequeue() {
		t.Errorf("failed to set requeue using SetDelayedRequeue")
		return false
	}
	/*if !k.delayed {
		t.Errorf("failed to set delayed using SetDelayedRequeue")
		return false
	}*/
	if !l.IsDelayed() {
		t.Errorf("failed to set delayed using SetDelayedRequeue")
		return false
	}
	return true
}

func TestSetDelayedRequeue(t *testing.T) {
	l, e := getLayer(emptyStatus, layersData)
	if e != nil {
		t.Fatalf("failed to create layer, error: %s", e.Error())
	}
	if !testDelayedRequeue(t, l) {
		return
	}
	if !testDelayedRequeue(t, l) { // Verify it works if set again when already set
		return
	}
}

func testRequeue(t *testing.T, l layers.Layer) bool {
	l.SetRequeue()
	/*k, ok := l.(*layers.KraanLayer)
	if !ok {
		t.Errorf("failed to cast layer interface to *layers.KraanLayer")
		return false
	}
	if !k.requeue {
		t.Errorf("failed to set requeue using SetRequeue")
		return false
	}*/
	if !l.NeedsRequeue() {
		t.Errorf("failed to set requeue using SetRequeue")
		return false
	}
	return true
}

func TestSetRequeue(t *testing.T) {
	l, e := getLayer(emptyStatus, layersData)
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

func testUpdated(t *testing.T, l layers.Layer) bool {
	l.SetUpdated()
	/*k, ok := l.(*layers.KraanLayer)
	if !ok {
		t.Errorf("failed to cast layer interface to *layers.KraanLayer")
		return false
	}
	if !k.updated {
		t.Errorf("failed to set updated using SetUpdated")
		return false
	}*/
	if !l.IsUpdated() {
		t.Errorf("failed to set updated using SetUpdated")
		return false
	}
	return true
}

func TestSetUpdated(t *testing.T) {
	l, e := getLayer(emptyStatus, layersData)
	if e != nil {
		t.Fatalf("failed to create layer, error: %s", e.Error())
	}
	if !testUpdated(t, l) {
		return
	}
	if !testUpdated(t, l) { // Verify it works if set again when already set
		return
	}
}

func compareConditions(actual, expected []kraanv1alpha1.Condition) error {
	if len(actual) != len(expected) {
		return fmt.Errorf("mismatch in number of conditions")
	}
	for index, condition := range actual {
		expect := expected[index]
		if condition.Message != expect.Message {
			return fmt.Errorf("mismatch in condition: %d, message...\nActual: %s\nExpected: %s",
				index, condition.Message, expect.Message)
		}
		if condition.Reason != expect.Reason {
			return fmt.Errorf("mismatch in condition: %d, reason...\nActual: %s\nExpected: %s",
				index, condition.Reason, expect.Reason)
		}
		if condition.Status != expect.Status {
			return fmt.Errorf("mismatch in condition: %d, status...\nActual: %s\nExpected: %s",
				index, condition.Status, expect.Status)
		}
		if condition.Type != expect.Type {
			return fmt.Errorf("mismatch in condition: %d, type..\nActual: %s\nExpected: %s",
				index, condition.Type, expect.Type)
		}
		if condition.Version != expect.Version {
			return fmt.Errorf("mismatch in condition: %d, version...\nActual: %s\nExpected: %s",
				index, condition.Version, expect.Version)
		}
	}
	return nil
}

func resetConditions(l layers.Layer) {
	l.GetFullStatus().Conditions = []kraanv1alpha1.Condition{}
}

func displayStatus(status *kraanv1alpha1.AddonsLayerStatus) string {
	statusJSON, err := testutils.ToJSON(status)
	if err != nil {
		return fmt.Sprintf("failed to generate json output for actual result, error: %s", err.Error())
	}

	return fmt.Sprintf("%s", statusJSON)
}

func compareStatus(actual, expected *kraanv1alpha1.AddonsLayerStatus) error {
	if actual.State != expected.State {
		return fmt.Errorf("mismatch in status state...\nActual: %s\nExpected: %s", actual.State, expected.State)
	}
	if actual.Version != expected.Version {
		return fmt.Errorf("mismatch in status version...\nActual: %s\nExpected: %s", actual.Version, expected.Version)
	}
	if err := compareConditions(actual.Conditions, expected.Conditions); err != nil {
		return fmt.Errorf("mismatch in status conditions...\nActual: %s\nExpected: %s\n\n%s",
			displayStatus(actual), displayStatus(expected), err.Error())
	}
	return nil
}

func TestSetStatusSetting(t *testing.T) { // nolint:funlen // ok
	type testsData struct {
		name     string
		setFunc  func()
		expected *kraanv1alpha1.AddonsLayerStatus
	}

	l, e := getLayer(emptyStatus, layersData)
	if e != nil {
		t.Fatalf("failed to create layer, error: %s", e.Error())
	}

	tests := []testsData{{
		name:    "SetStatusK8sVersion",
		setFunc: l.SetStatusK8sVersion,
		expected: &kraanv1alpha1.AddonsLayerStatus{
			State:   kraanv1alpha1.K8sVersionCondition,
			Version: versionOne,
			Conditions: []kraanv1alpha1.Condition{{
				Status:  corev1.ConditionTrue,
				Version: versionOne,
				Type:    kraanv1alpha1.K8sVersionCondition,
				Reason:  kraanv1alpha1.AddonsLayerK8sVersionReason,
				Message: kraanv1alpha1.AddonsLayerK8sVersionMsg},
			},
		}}, {
		name:    "SetStatusPruning",
		setFunc: l.SetStatusPruning,
		expected: &kraanv1alpha1.AddonsLayerStatus{
			State:   kraanv1alpha1.PruningCondition,
			Version: versionOne,
			Conditions: []kraanv1alpha1.Condition{{
				Status:  corev1.ConditionTrue,
				Version: versionOne,
				Type:    kraanv1alpha1.PruningCondition,
				Reason:  kraanv1alpha1.AddonsLayerPruningReason,
				Message: kraanv1alpha1.AddonsLayerPruningMsg},
			},
		}}, {
		name:    "SetStatusApplying",
		setFunc: l.SetStatusApplying,
		expected: &kraanv1alpha1.AddonsLayerStatus{
			State:   kraanv1alpha1.ApplyingCondition,
			Version: versionOne,
			Conditions: []kraanv1alpha1.Condition{{
				Status:  corev1.ConditionTrue,
				Version: versionOne,
				Type:    kraanv1alpha1.ApplyingCondition,
				Reason:  kraanv1alpha1.AddonsLayerApplyingReason,
				Message: kraanv1alpha1.AddonsLayerApplyingMsg},
			},
		}}, {
		name:    "SetStatusDeployed",
		setFunc: l.SetStatusDeployed,
		expected: &kraanv1alpha1.AddonsLayerStatus{
			State:   kraanv1alpha1.DeployedCondition,
			Version: versionOne,
			Conditions: []kraanv1alpha1.Condition{{
				Status:  corev1.ConditionTrue,
				Version: versionOne,
				Type:    kraanv1alpha1.DeployedCondition,
				Reason:  "AddonsLayer version 0.1.01 is Deployed",
				Message: ""},
			},
		}},
	}

	for _, test := range tests {
		resetConditions(l)
		test.setFunc()
		if err := compareStatus(l.GetFullStatus(), test.expected); err != nil {
			t.Fatalf("test: %s, failed, error: %s", test.name, err.Error())
		}
	}
}

func TestSetStatusUpdate(t *testing.T) {
	type testsData struct {
		status   string
		reason   string
		message  string
		expected *kraanv1alpha1.AddonsLayerStatus
	}

	const (
		reason  = "the reason"
		message = "the message"
	)

	l, e := getLayer(emptyStatus, layersData)
	if e != nil {
		t.Fatalf("failed to create layer, error: %s", e.Error())
	}

	tests := []testsData{{
		status:  kraanv1alpha1.ApplyingCondition,
		reason:  reason,
		message: message,
		expected: &kraanv1alpha1.AddonsLayerStatus{
			State:   kraanv1alpha1.ApplyingCondition,
			Version: versionOne,
			Conditions: []kraanv1alpha1.Condition{{
				Status:  corev1.ConditionTrue,
				Version: versionOne,
				Type:    kraanv1alpha1.ApplyingCondition,
				Reason:  reason,
				Message: message},
			},
		}},
	}

	for number, test := range tests {
		resetConditions(l)
		l.StatusUpdate(test.status, test.reason, test.message)
		if err := compareStatus(l.GetFullStatus(), test.expected); err != nil {
			t.Fatalf("test: %d, failed, error: %s", number+1, err.Error())
		}
	}
}

func TestHold(t *testing.T) {
	type testsData struct {
		layerName string
		expected  bool
		status    *kraanv1alpha1.AddonsLayerStatus
	}
	tests := []testsData{{
		layerName: emptyStatus,
		expected:  false,
		status:    &kraanv1alpha1.AddonsLayerStatus{},
	}, {
		layerName: holdSet,
		expected:  true,
		status: &kraanv1alpha1.AddonsLayerStatus{
			State:   kraanv1alpha1.HoldCondition,
			Version: versionOne,
			Conditions: []kraanv1alpha1.Condition{{
				Status:  corev1.ConditionTrue,
				Version: versionOne,
				Type:    kraanv1alpha1.HoldCondition,
				Reason:  kraanv1alpha1.AddonsLayerHoldReason,
				Message: kraanv1alpha1.AddonsLayerHoldMsg},
			},
		}},
	}
	for number, test := range tests {
		l, e := getLayer(test.layerName, layersData)
		if e != nil {
			t.Fatalf("failed to create layer, error: %s", e.Error())
		}
		if l.IsHold() != test.expected {
			t.Fatalf("expected hold to be %t", test.expected)
		}
		l.SetHold()
		if err := compareStatus(l.GetFullStatus(), test.status); err != nil {
			t.Fatalf("test: %d, failed", number+1)
		}
	}
}

/*func TestSetStatus(t *testing.T) { // nolint:funlen // ok
	type testsData struct {
		name      string
		layerName string
		status    string
		reason    string
		message   string
		expected  *kraanv1alpha1.AddonsLayerStatus
	}

	tests := []testsData{{
		name:      "set status adding a condition when there is an existing condition",
		layerName: k8sPending,
		status:    kraanv1alpha1.PruningCondition,
		reason:    kraanv1alpha1.AddonsLayerPruningReason,
		message:   kraanv1alpha1.AddonsLayerPruningMsg,
		expected: &kraanv1alpha1.AddonsLayerStatus{
			State:   kraanv1alpha1.PruningCondition,
			Version: versionOne,
			Conditions: []kraanv1alpha1.Condition{{
				Status:  corev1.ConditionFalse,
				Version: versionOne,
				Type:    kraanv1alpha1.K8sVersionCondition,
				Reason:  kraanv1alpha1.AddonsLayerK8sVersionReason,
				Message: kraanv1alpha1.AddonsLayerK8sVersionMsg},
				{
					Status:  corev1.ConditionTrue,
					Version: versionOne,
					Type:    kraanv1alpha1.PruningCondition,
					Reason:  kraanv1alpha1.AddonsLayerPruningReason,
					Message: kraanv1alpha1.AddonsLayerPruningMsg},
			},
		}}, {
		name:      "set status when no existing status",
		layerName: emptyStatus,
		status:    kraanv1alpha1.PruningCondition,
		reason:    kraanv1alpha1.AddonsLayerPruningReason,
		message:   kraanv1alpha1.AddonsLayerPruningMsg,
		expected: &kraanv1alpha1.AddonsLayerStatus{
			State:   kraanv1alpha1.PruningCondition,
			Version: versionOne,
			Conditions: []kraanv1alpha1.Condition{{
				Status:  corev1.ConditionTrue,
				Version: versionOne,
				Type:    kraanv1alpha1.PruningCondition,
				Reason:  kraanv1alpha1.AddonsLayerPruningReason,
				Message: kraanv1alpha1.AddonsLayerPruningMsg},
			},
		}}, {
		name:      "set status when no existing status is same",
		layerName: k8sPending,
		status:    kraanv1alpha1.K8sVersionCondition,
		reason:    kraanv1alpha1.AddonsLayerK8sVersionReason,
		message:   kraanv1alpha1.AddonsLayerK8sVersionMsg,
		expected: &kraanv1alpha1.AddonsLayerStatus{
			State:   kraanv1alpha1.K8sVersionCondition,
			Version: versionOne,
			Conditions: []kraanv1alpha1.Condition{{
				Status:  corev1.ConditionTrue,
				Version: versionOne,
				Type:    kraanv1alpha1.K8sVersionCondition,
				Reason:  kraanv1alpha1.AddonsLayerK8sVersionReason,
				Message: kraanv1alpha1.AddonsLayerK8sVersionMsg},
			},
		}}, {
		name:      "set status when maximum number of conditions already",
		layerName: maxConditions,
		status:    kraanv1alpha1.PruningCondition,
		reason:    kraanv1alpha1.AddonsLayerPruningReason,
		message:   kraanv1alpha1.AddonsLayerPruningMsg,
		expected: &kraanv1alpha1.AddonsLayerStatus{
			State:   kraanv1alpha1.PruningCondition,
			Version: versionOne,
			Conditions: []kraanv1alpha1.Condition{
				{
					Status:  corev1.ConditionFalse,
					Version: versionOne,
					Type:    kraanv1alpha1.PruningCondition,
					Reason:  kraanv1alpha1.AddonsLayerPruningReason,
					Message: kraanv1alpha1.AddonsLayerPruningMsg},
				{
					Status:  corev1.ConditionFalse,
					Version: versionOne,
					Type:    kraanv1alpha1.ApplyPendingCondition,
					Reason:  "waiting for layer: test-layer2, version: 0.1.01 to be applied.",
					Message: "Layer: test-layer2, current state: Applying."},
				{
					Status:  corev1.ConditionFalse,
					Version: versionOne,
					Type:    kraanv1alpha1.ApplyingCondition,
					Reason:  kraanv1alpha1.AddonsLayerApplyingReason,
					Message: kraanv1alpha1.AddonsLayerApplyingMsg},
				{
					Status:  corev1.ConditionFalse,
					Version: versionOne,
					Type:    kraanv1alpha1.DeployedCondition,
					Reason:  kraanv1alpha1.AddonsLayerDeployedReason,
					Message: ""},
				{
					Status:  corev1.ConditionFalse,
					Version: versionOne,
					Type:    kraanv1alpha1.K8sVersionCondition,
					Reason:  kraanv1alpha1.AddonsLayerK8sVersionReason,
					Message: kraanv1alpha1.AddonsLayerK8sVersionMsg},
				{
					Status:  corev1.ConditionFalse,
					Version: versionOne,
					Type:    kraanv1alpha1.PruningCondition,
					Reason:  kraanv1alpha1.AddonsLayerPruningReason,
					Message: kraanv1alpha1.AddonsLayerPruningMsg},
				{
					Status:  corev1.ConditionFalse,
					Version: versionOne,
					Type:    kraanv1alpha1.ApplyPendingCondition,
					Reason:  "waiting for layer: test-layer2, version: 0.1.01 to be applied.",
					Message: "Layer: test-layer2, current state: Applying."},
				{
					Status:  corev1.ConditionFalse,
					Version: versionOne,
					Type:    kraanv1alpha1.ApplyingCondition,
					Reason:  kraanv1alpha1.AddonsLayerApplyingReason,
					Message: kraanv1alpha1.AddonsLayerApplyingMsg},
				{
					Status:  corev1.ConditionFalse,
					Version: versionOne,
					Type:    kraanv1alpha1.DeployedCondition,
					Reason:  kraanv1alpha1.AddonsLayerDeployedReason,
					Message: ""},
				{
					Status:  corev1.ConditionTrue,
					Version: versionOne,
					Type:    kraanv1alpha1.PruningCondition,
					Reason:  kraanv1alpha1.AddonsLayerPruningReason,
					Message: kraanv1alpha1.AddonsLayerPruningMsg},
			}}}}

	for _, test := range tests {
		l, e := getLayer(test.layerName, layersData)
		if e != nil {
			t.Fatalf("test: %s, failed to create layer, error: %s", test.name, e.Error())
		}
		l.setStatus(test.status, test.reason, test.message)
		if err := compareStatus(l.GetFullStatus(), test.expected); err != nil {
			t.Fatalf("test: %s, failed, error: %s", test.name, err.Error())
		}
		t.Logf("test: %s, successful", test.name)
	}
}*/

func TestCheckK8sVersion(t *testing.T) { // nolint:funlen // ok
	type testsData struct {
		name       string
		layerName  string
		k8sVersion string
		expected   bool
		errorFunc  fakeTest.ReactionFunc
	}

	tests := []testsData{{
		name:       "check k8s version v1.18 required, cluster at v1.16",
		layerName:  k8sPending,
		k8sVersion: "v1.16.2",
		expected:   false,
	}, {
		name:       "check k8s version, v1.16 required. cluster at v1.16.9",
		layerName:  k8sv16,
		k8sVersion: "v1.16.9",
		expected:   true,
	}, {
		name:       "check k8s version, v1.16 required. cluster at v1.15",
		layerName:  k8sv16,
		k8sVersion: "v1.15.0",
		expected:   false,
	}, {
		name:       "check k8s version, v1.16.2 required. cluster at v1.16.1",
		layerName:  k8sv16_2,
		k8sVersion: "v1.15.0",
		expected:   false,
	},
	/*
		This test should return false and an error but there seems to be a bug in the fake testing impementation,
		see https://github.com/kubernetes/client-go/issues/858
		{
		name:       "check error getting server version",
		layerName:  k8sv16,
		k8sVersion: "v1.18.0",
		expected:   true,
		errorFunc: func(action fakeTest.Action) (handled bool, ret runtime.Object, err error) {
			return true, nil, fmt.Errorf("error")
		},
	*/
	}

	for _, test := range tests {
		l, e := getLayer(test.layerName, layersData)
		if e != nil {
			t.Fatalf("test: %s, failed to create layer, error: %s", test.name, e.Error())
		}
		fakeD, ok := fakeK8sClient.Discovery().(*fakediscovery.FakeDiscovery)
		if !ok {
			t.Fatalf("test: %s, failed, couldn't convert Discovery() to *FakeDiscovery", test.name)
		}
		if test.errorFunc != nil {
			fakeD.FakedServerVersion = nil
			fakeD.Fake.AddReactor("get", "version", test.errorFunc)
		} else {
			fakeD.FakedServerVersion = &version.Info{GitVersion: test.k8sVersion}
		}
		result := l.CheckK8sVersion()
		if result != test.expected {
			t.Fatalf("test: %s, failed, wrong result, Actual: %t, Expected: %t", test.name, result, test.expected)
		}
		t.Logf("test: %s, successful", test.name)
	}
}

func TestDependenciesDeployed(t *testing.T) {
	type testsData struct {
		name       string
		layerName  string
		layersData string
		expected   bool
	}

	tests := []testsData{{
		name:       "check dependencies with no dependsOn",
		layerName:  noDepends,
		layersData: layersData1,
		expected:   true,
	}, {
		name:       "check dependencies with single dependsOn that is deployed",
		layerName:  oneDepends,
		layersData: layersData1,
		expected:   true,
	}, {
		name:       "check dependencies with single dependsOn that is deployed but previous version",
		layerName:  oneDependsV2,
		layersData: layersData1,
		expected:   false,
	}, {
		name:       "check dependencies with two dependsOn, both deployed",
		layerName:  twoDepends,
		layersData: layersData1,
		expected:   true,
	}, {
		name:       "check dependencies with single dependsOn that is not deployed",
		layerName:  oneDepends,
		layersData: layersData2,
		expected:   false,
	}, {
		name:       "check dependencies with two dependsOn, second not deployed",
		layerName:  twoDepends,
		layersData: layersData2,
		expected:   false,
	},
	}

	for _, test := range tests {
		l, e := getLayer(test.layerName, test.layersData)
		if e != nil {
			t.Fatalf("test: %s, failed to create layer, error: %s", test.name, e.Error())
		}
		result := l.DependenciesDeployed()
		if result != test.expected {
			t.Fatalf("test: %s, failed, wrong result, Actual: %t, Expected: %t", test.name, result, test.expected)
		}
		t.Logf("test: %s, successful", test.name)
	}
}
