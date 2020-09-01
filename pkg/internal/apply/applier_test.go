package apply

import (
	"testing"
	"time"

	kraanv1alpha1 "github.com/fidelity/kraan/pkg/api/v1alpha1"
	"github.com/fidelity/kraan/pkg/internal/kubectl"

	helmopv1 "github.com/fluxcd/helm-operator/pkg/apis/helm.fluxcd.io/v1"

	"github.com/go-logr/logr"
	testlogr "github.com/go-logr/logr/testing"
	gomock "github.com/golang/mock/gomock"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	types "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	testScheme = runtime.NewScheme()
)

func init() {
	_ = corev1.AddToScheme(testScheme)        // nolint:errcheck // ok
	_ = kraanv1alpha1.AddToScheme(testScheme) // nolint:errcheck // ok
	_ = helmopv1.AddToScheme(testScheme)      // nolint:errcheck // ok
}

func fakeAddonsLayer(sourcePath, layerName string, layerUID types.UID) *kraanv1alpha1.AddonsLayer {
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

	mockKubectl := kubectl.NewMockKubectl(mockCtl)
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
