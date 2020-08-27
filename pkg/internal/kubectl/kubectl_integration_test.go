// +build integration

package kubectl

/*

The mockgen tool generates the MockExecProvider type defined in the kubectl/mockExecProvider.go code file.

From the project root directory, you can generate mock definitions for interfaces in individual code files by calling mockgen.  Example:
	mockgen -destination=pkg/internal/kubectl/mockExecProvider.go
		-package=kubectl -source=pkg/internal/kubectl/execProvider.go
		gitlab.fmr.com/common-platform/addons-manager/pkg/internal/kubectl ExecProvider

Or you can generate all the

Add a go:generate annotation above the package statement in all the code files containing interfaces that you want to mock.  Example:
//go:generate mockgen -destination=mockExecProvider.go -package=kubectl -source=execProvider.go . ExecProvider
//go:generate mockgen -destination=../mocks/logr/mockLogger.go -package=mocks github.com/go-logr/logr Logger

From the project root directory, you can then generate mocks for all the interfaces that have a go:generate annotation by running 'go generate ./...'.

*/
import (
	"fmt"
	"testing"

	testlogr "github.com/go-logr/logr/testing"
)

func TestRealKubectlBinaryInstalled(t *testing.T) {
	logger := testlogr.TestLogger{T: t}
	k, err := NewKubectl(logger)
	t.Logf("Kubectl (%T) %#v", k, k)
	if err != nil {
		t.Errorf("Error from NewKubectl constructor function: %w", err)
	} else {
		t.Logf("Found '%s' binary at '%s'", kubectlCmd, k.getPath())
	}
}

func TestRealOtherBinaryNotInstalled(t *testing.T) {
	restoreCmd := kubectlCmd
	defer func() { kubectlCmd = restoreCmd }()

	kubectlCmd = "not-kubectl-and-not-installed"

	logger := testlogr.TestLogger{T: t}
	k, err := NewKubectl(logger)
	t.Logf("Kubectl (%T) %#v", k, k)
	if err == nil {
		foundCmdMsg := fmt.Sprintf("Found '%s' binary at '%s'", kubectlCmd, k.getPath())
		t.Errorf("Expected error 'executable file not found' was not returned from NewKubectl constructor: %s", foundCmdMsg)
	} else {
		t.Logf("Expected error was returned: %#v", err)
	}
}

func TestSimpleApply(t *testing.T) {
	logger := testlogr.TestLogger{T: t}
	k, err := NewKubectl(logger)
	t.Logf("Kubectl (%T) %#v", k, k)
	if err != nil {
		t.Fatalf("Error from NewKubectl constructor function: %s", err)
	}
	out, err := k.Apply("testdata/apply/simpleapply").Run()
	if err != nil {
		t.Fatalf("Error from Kubectl Apply.Run function: %s", err)
	}
	t.Logf("Output: %s", out)

	out, err = k.Get("namespace", "simple", "-o", "yaml").Run()
	if err != nil {
		t.Fatalf("Error from Kubectl Get.Run function: %s", err)
	}
	t.Logf("Output: %s", out)

	out, err = k.Delete("-f", "testdata/apply/simpleapply").Run()
	if err != nil {
		t.Fatalf("Error from Kubectl Delete.Run function: %s", err)
	}
	t.Logf("Output: %s", out)

	out, err = k.Get("namespace", "simple", "-o", "yaml").Run()
	if err == nil {
		t.Fatalf("Kubectl Delete.Run function failed to delete the test resource")
	}
	t.Logf("Output: %s", out)
}
