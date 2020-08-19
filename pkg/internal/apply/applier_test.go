package apply

import (
	"testing"

	hoscheme "github.com/fluxcd/helm-operator/pkg/client/clientset/versioned/scheme"

	"github.com/fidelity/kraan/pkg/internal/kubectl"
	"github.com/go-logr/logr"
	testlogr "github.com/go-logr/logr/testing"
	gomock "github.com/golang/mock/gomock"
	corescheme "k8s.io/client-go/kubernetes/scheme"
)

func init() {
	hoscheme.AddToScheme(corescheme.Scheme)
}

func fakeLogger() logr.Logger {
	return testlogr.NullLogger{}
}

func testLogger(t *testing.T) logr.Logger {
	return testlogr.TestLogger{T: t}
}

func TestNewApplier(t *testing.T) {
	logger := fakeLogger()
	applier, err := NewApplier(logger)
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

	logger := fakeLogger()
	applier, err := NewApplier(logger)
	if err != nil {
		t.Fatalf("The NewApplier constructor returned an error: %s", err)
	}
	t.Logf("NewApplier returned (%T) %#v", applier, applier)
}
