package apply // nolint:package // unittest code should be in same package

import (
	"testing"

	kraanscheme "github.com/fidelity/kraan/pkg/api/v1alpha1"
	"github.com/fidelity/kraan/pkg/internal/kubectl"

	hrscheme "github.com/fluxcd/helm-operator/pkg/client/clientset/versioned/scheme"

	"github.com/go-logr/logr"
	testlogr "github.com/go-logr/logr/testing"
	gomock "github.com/golang/mock/gomock"

	"k8s.io/apimachinery/pkg/runtime"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	testScheme = runtime.NewScheme()
)

func init() {
	_ = k8sscheme.AddToScheme(testScheme)   // nolint:errcheck // ok
	_ = kraanscheme.AddToScheme(testScheme) // nolint:errcheck // ok
	_ = hrscheme.AddToScheme(testScheme)    // nolint:errcheck // ok
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
