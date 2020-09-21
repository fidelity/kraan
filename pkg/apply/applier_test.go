package apply

import (
	"context"
	"testing"
	"time"

	helmopv1 "github.com/fluxcd/helm-operator/pkg/apis/helm.fluxcd.io/v1"
	"github.com/go-logr/logr"
	testlogr "github.com/go-logr/logr/testing"
	gomock "github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	types "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kraanv1alpha1 "github.com/fidelity/kraan/api/v1alpha1"
	"github.com/fidelity/kraan/pkg/internal/kubectl"
	mocks "github.com/fidelity/kraan/pkg/internal/mocks/client"
	kubectlmocks "github.com/fidelity/kraan/pkg/internal/mocks/kubectl"
	"github.com/fidelity/kraan/pkg/layers"
)

var (
	testScheme = runtime.NewScheme()
)

func init() {
	_ = corev1.AddToScheme(testScheme)        // nolint:errcheck // ok
	_ = kraanv1alpha1.AddToScheme(testScheme) // nolint:errcheck // ok
	_ = helmopv1.AddToScheme(testScheme)      // nolint:errcheck // ok
}

func fakeAddonsLayer(sourcePath, layerName string, layerUID types.UID) *kraanv1alpha1.AddonsLayer { //nolint
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
	sourceSpec := kraanv1alpha1.SourceSpec{
		Name: "TestingSource",
		Path: sourcePath,
	}
	layerPreReqs := kraanv1alpha1.PreReqs{
		K8sVersion: "1.15.3",
		//K8sVersion string `json:"k8sVersion"`
		//DependsOn []string `json:"dependsOn,omitempty"`
	}
	layerSpec := kraanv1alpha1.AddonsLayerSpec{
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
	layerStatus := kraanv1alpha1.AddonsLayerStatus{
		State:   "Testing",
		Version: "v1alpha1",
		//Conditions []Condition `json:"conditions,omitempty"`
		//State string `json:"state,omitempty"`
		//Version string `json:"version,omitempty"`
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
	applier, err := NewApplier(client, logger, testScheme)
	if err != nil {
		t.Fatalf("The NewApplier constructor returned an error: %s", err)
	}
	t.Logf("NewApplier returned (%T) %#v", applier, applier)
}

func TestMockKubectl(t *testing.T) {
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	mockKubectl := kubectlmocks.NewMockKubectl(mockCtl)
	newKubectlFunc = func(logger logr.Logger) (kubectl.Kubectl, error) {
		return mockKubectl, nil
	}

	logger := testlogr.TestLogger{T: t}
	client := fake.NewFakeClientWithScheme(testScheme)
	applier, err := NewApplier(client, logger, testScheme)
	if err != nil {
		t.Fatalf("The NewApplier constructor returned an error: %s", err)
	}
	t.Logf("NewApplier returned (%T) %#v", applier, applier)
}

func TODOTestBasicApply(t *testing.T) { //nolint
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	mockCommand := kubectlmocks.NewMockCommand(mockCtl)
	mockKubectl := kubectlmocks.NewMockKubectl(mockCtl)
	newKubectlFunc = func(logger logr.Logger) (kubectl.Kubectl, error) {
		return mockKubectl, nil
	}

	ctx := context.Background()
	logger := testlogr.TestLogger{T: t}
	client := mocks.NewMockClient(mockCtl)

	applier, err := NewApplier(client, logger, testScheme)
	if err != nil {
		t.Fatalf("The NewApplier constructor returned an error: %s", err)
	}
	t.Logf("NewApplier returned (%T) %#v", applier, applier)

	// This integration test can be forced to pass or fail at different stages by altering the
	// Values section of the microservice.yaml HelmRelease in the directory below.
	sourcePath := "testdata/apply/single_release"
	layerName := "test"
	var layerUID types.UID = "01234567-89ab-cdef-0123-456789abcdef"
	addonsLayer := fakeAddonsLayer(sourcePath, layerName, layerUID)

	//fakeHr := &helmopv1.HelmRelease{}
	// TODO - serailize a fake HelmRelease
	//sez := serializer.NewCodecFactory(testScheme).Co
	fakeHrJSON := "fakeHrJson"

	mockKubectl.EXPECT().Apply(sourcePath).Return(mockCommand).Times(1)
	mockCommand.EXPECT().WithLogger(logger).Return(mockCommand).Times(1)
	mockCommand.EXPECT().DryRun().Return(fakeHrJSON, nil).Times(1)

	mockLayer := layers.NewMockLayer(mockCtl)
	mockLayer.EXPECT().GetName().Return(layerName).AnyTimes()
	mockLayer.EXPECT().GetSourcePath().Return(sourcePath).AnyTimes()
	mockLayer.EXPECT().GetLogger().Return(logger).AnyTimes()
	mockLayer.EXPECT().GetAddonsLayer().Return(addonsLayer).Times(1)

	err = applier.Apply(ctx, mockLayer)
	if err != nil {
		t.Fatalf("LayerApplier.Apply returned an error: %s", err)
	}
}
