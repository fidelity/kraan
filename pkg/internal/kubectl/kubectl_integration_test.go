package kubectl

/*

The mockgen tool generates the MockExecProvider type defined in the kubectl/mockExecProvider.go code file.

From the project root directory, you can generate mock definitions for interfaces in individual code files by calling mockgen.  Example:
	mockgen -destination=pkg/internal/kubectl/mockExecProvider.go -package=kubectl -source=pkg/internal/kubectl/execProvider.go gitlab.fmr.com/common-platform/addons-manager/pkg/internal/kubectl ExecProvider

Or you can generate all the

Add a go:generate annotation above the package statement in all the code files containing interfaces that you want to mock.  Example:
//go:generate mockgen -destination=mockExecProvider.go -package=kubectl -source=execProvider.go . ExecProvider
//go:generate mockgen -destination=../mocks/logr/mockLogger.go -package=mocks github.com/go-logr/logr Logger

From the project root directory, you can then generate mocks for all the interfaces that have a go:generate annotation by running 'go generate ./...'.

*/
import (
	//"bytes"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	gomock "github.com/golang/mock/gomock"
)

func TestRealKubectlBinaryInstalled(t *testing.T) {
	k, err := NewKubectl(fakeLogger())
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

	k, err := NewKubectl(fakeLogger())
	t.Logf("Kubectl (%T) %#v", k, k)
	if err == nil {
		foundCmdMsg := fmt.Sprintf("Found '%s' binary at '%s'", kubectlCmd, k.getPath())
		t.Errorf("Expected error 'executable file not found' was not returned from NewKubectl constructor: %s", foundCmdMsg)
	} else {
		t.Logf("Expected error was returned: %#v", err)
	}
}

func TestGetPwd(t *testing.T) {
	out, err := exec.Command("ls", "-alhF", "../../../vendor").CombinedOutput()
	if err != nil {
		t.Fatalf("errorreturned from pwd : %s", err)
	}
	t.Logf("pwd returned [%s]", out)
}

func TestSimpleApply(t *testing.T) {
	k, err := NewKubectl(fakeLogger())
	t.Logf("Kubectl (%T) %#v", k, k)
	if err != nil {
		t.Fatalf("Error from NewKubectl constructor function: %s", err)
	}
	//out, err := k.Apply("../../../test/testdata/kubectl/apply/simpleapply").Run()
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

	//out, err = k.Get("namespace").Run()
	out, err = k.Get("namespace", "simple", "-o", "yaml").Run()
	if err == nil {
		t.Fatalf("Kubectl Delete.Run function failed to delete the test resource")
	}
	t.Logf("Output: %s", out)
}

func TestMockKubectlApplyReturnsApplyCommand(t *testing.T) {
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	restoreExecProvider := execProvider
	defer func() { execProvider = restoreExecProvider }()

	mockExecProvider := NewMockExecProvider(mockCtl)
	execProvider = mockExecProvider

	fakeCommandPath := "/mocked/path/to/kubectl/binary"
	// Verifies that the "findOnPath" method was called once with the exact expected string input as the parameter
	mockExecProvider.EXPECT().findOnPath(kubectlCmd).Return(fakeCommandPath, nil).Times(1)

	k, err := NewKubectl(fakeLogger())
	t.Logf("Kubectl (%T) %#v", k, k)
	if err != nil {
		t.Errorf("Error returned from the execLookPath function : %w", err)
	}

	fakeSourceDir := "/mocked/path/to/source/directory"
	expectedSubCmd := "apply"
	expectedArgs := []string{expectedSubCmd, "-R", "-f", fakeSourceDir}
	expectedCmd := fmt.Sprintf("%s %s", fakeCommandPath, strings.Join(expectedArgs, " "))

	var c interface{} = k.Apply(fakeSourceDir)
	if c == nil {
		t.Fatalf("nil returned from Kubectl.Apply(sourcedir) function")
	}
	t.Logf("Returned (%T) %#v", c, c)
	switch c.(type) {
	case *ApplyCommand:
		t.Logf("ApplyCommand (%T) %#v", c, c)
	default:
		t.Fatalf("return value from Kubectl.Apply is not an ApplyCommand type: (%T) %#v", c, c)
	}
	a := c.(*ApplyCommand)

	// verify that the returned ApplyCommand.kubectlSubCmd matches expectations
	if expectedSubCmd != a.kubectlSubCmd() {
		t.Fatalf("expected '%s', got '%s' for the returned ApplyCommand.kubectlSubCmd", expectedSubCmd, a.kubectlSubCmd())
	} else {
		t.Logf("kubectlSubCmd expected '%s' matches '%s'", expectedSubCmd, a.kubectlSubCmd())
	}

	// verify that the returned ApplyCommand.kubectlArgs match expectations
	if len(a.kubectlArgs()) != len(expectedArgs) {
		t.Fatalf("expected %d args, got %d args in the returned ApplyCommand", len(expectedArgs), len(a.args))
	}
	argsEqual := true
	for i, arg := range a.kubectlArgs() {
		expectedArg := expectedArgs[i]
		if arg != expectedArg {
			t.Errorf("expected arg '%s', got arg '%s' at index [%d] in the returned ApplyCommand", expectedArg, arg, i)
			argsEqual = false
		} else {
			t.Logf("at index [%d] expected arg '%s' matches arg '%s'", i, expectedArg, arg)
		}
	}
	if !argsEqual {
		t.Fatalf("args in the returned ApplyCommand did not match expectations")
	}

	// verify that the command path in the returned ApplyCommand matches expectations
	if fakeCommandPath != a.kubectlPath() {
		t.Fatalf("expected '%s', got '%s' for kubectlPath in the returned ApplyCommand", fakeCommandPath, a.kubectlPath())
	} else {
		t.Logf("kubectlPath expected '%s' matches '%s'", fakeCommandPath, a.kubectlPath())
	}

	// verify that asString concatenates the kubectlCommand and args slice in order
	if expectedCmd != a.asString() {
		t.Fatalf("expected '%s', got '%s' from the returned ApplyCommand asString function", expectedCmd, a.asString())
	} else {
		t.Logf("asString expected '%s' matches '%s'", expectedCmd, a.asString())
	}
}

func TestMockKubectlApplyRun(t *testing.T) {
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	restoreExecProvider := execProvider
	defer func() { execProvider = restoreExecProvider }()

	mockExecProvider := NewMockExecProvider(mockCtl)
	execProvider = mockExecProvider

	fakeCommandPath := "/mocked/path/to/kubectl"
	expectedSubCmd := "apply"
	fakeSourceDir := "/mocked/path/to/source/directory"
	expectedArgs := []string{expectedSubCmd, "-R", "-f", fakeSourceDir, "-o", "json"}
	expectedOutput := ""
	// Verifies that the "findOnPath" method was called once with the exact expected string input as the parameter
	mockExecProvider.EXPECT().findOnPath(kubectlCmd).Return(fakeCommandPath, nil).Times(1)
	mockExecProvider.EXPECT().execCmd(fakeCommandPath, expectedArgs).Return([]byte(expectedOutput), fmt.Errorf("MOCK error executing command")).Times(1)

	k, err := NewKubectl(fakeLogger())
	t.Logf("Kubectl (%T) %#v", k, k)
	if err != nil {
		t.Errorf("Error returned from the execLookPath function : %w", err)
	}

	c := k.Apply(fakeSourceDir)
	if c == nil {
		t.Fatalf("nil returned from Kubectl.Apply(sourcedir) function")
	}
	t.Logf("Returned (%T) %#v", c, c)

	_, err = c.Run()
	if err == nil {
		t.Fatalf("Expected error was not returned from ApplyCommand.Run")
	} else {
		t.Logf("Expected error was returned: %#v", err)
	}
}
