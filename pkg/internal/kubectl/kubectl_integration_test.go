// +build integration

package kubectl_test

import (
	"fmt"
	"testing"

	"github.com/fidelity/kraan/pkg/internal/kubectl"

	testlogr "github.com/go-logr/logr/testing"
)

func TestRealKubectlBinaryInstalled(t *testing.T) {
	logger := testlogr.TestLogger{T: t}
	k, err := kubectl.NewKubectl(logger)
	t.Logf("Kubectl (%T) %#v", k, k)
	if err != nil {
		t.Errorf("Error from NewKubectl constructor function: %w", err)
	} else {
		t.Logf("Found '%s' binary at '%s'", kubectl.KubectlCmd, kubectl.GetFactoryPath(*k.(*kubectl.CommandFactory)))
	}
}

func TestRealOtherBinaryNotInstalled(t *testing.T) {
	restoreCmd := kubectl.KubectlCmd
	defer kubectl.SetKubectlCmd(restoreCmd)

	kubectl.SetKubectlCmd("not-kubectl-and-not-installed")

	logger := testlogr.TestLogger{T: t}
	k, err := kubectl.NewKubectl(logger)
	t.Logf("Kubectl (%T) %#v", k, k)
	if err == nil {
		foundCmdMsg := fmt.Sprintf("Found '%s' binary at '%s'", kubectl.KubectlCmd, kubectl.GetFactoryPath(*k.(*kubectl.CommandFactory)))
		t.Errorf("Expected error 'executable file not found' was not returned from NewKubectl constructor: %s", foundCmdMsg)
	} else {
		t.Logf("Expected error was returned: %#v", err)
	}
}

func TestSimpleApply(t *testing.T) {
	logger := testlogr.TestLogger{T: t}
	k, err := kubectl.NewKubectl(logger)
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
