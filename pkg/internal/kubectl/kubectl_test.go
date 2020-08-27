package kubectl // Xnolint:package // unit tests should be in same package as code under test

/*

The mockgen tool generates the MockExecProvider type defined in the kubectl/mockExecProvider.go code file.

From the project root directory, you can generate mock definitions for interfaces in individual code files by calling mockgen.  Example:
	mockgen -destination=pkg/internal/kubectl/mockExecProvider.go -package=kubectl -source=pkg/internal/kubectl/execProvider.go \
	github.com/fidelity/kraan/pkg/internal/kubectl ExecProvider

Or you can generate all the

Add a go:generate annotation above the package statement in all the code files containing interfaces that you want to mock.  Example:
//go:generate mockgen -destination=mockExecProvider.go -package=kubectl -source=execProvider.go . ExecProvider
//go:generate mockgen -destination=../mocks/logr/mockLogger.go -package=mocks github.com/go-logr/logr Logger

From the project root directory, you can then generate mocks for all the interfaces that have a go:generate annotation by running 'go generate ./...'.

*/
import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	testlogr "github.com/go-logr/logr/testing"
	gomock "github.com/golang/mock/gomock"

	mocklogr "github.com/fidelity/kraan/pkg/internal/mocks/logr"
)

func TestNewKubectl(t *testing.T) {
	logger := testlogr.TestLogger{T: t}
	k, err := NewKubectl(logger)
	if err != nil {
		t.Errorf("The NewKubectl function returned an error! %w", err)
	}
	t.Logf("k (%T) %#v", k, k)
	gotLogger := k.getLogger()
	t.Logf("gotLogger (%T) %#v", gotLogger, gotLogger)
	if logger != gotLogger {
		t.Errorf("The passed logger was not stored as the new Kubectl instance's Logger")
	}
}

type FakeExecProvider struct {
	findOnPathFunc func(file string) (string, error)
	execCmdFunc    func(name string, arg ...string) ([]byte, error)
}

func (p FakeExecProvider) findOnPath(file string) (string, error) {
	return p.findOnPathFunc(file)
}

func (p FakeExecProvider) execCmd(name string, arg ...string) ([]byte, error) {
	return p.execCmdFunc(name, arg...)
}

func (p FakeExecProvider) setFindOnPathFunc(mockFunc func(file string) (string, error)) FakeExecProvider {
	p.findOnPathFunc = mockFunc
	return p
}

/* Not used
func (p FakeExecProvider) setExecCmdFunc(mockFunc func(name string, arg ...string) ([]byte, error)) FakeExecProvider {
	p.execCmdFunc = mockFunc
	return p
}
*/
// BaseFakeExecProvider implements a fake exec.
func BaseFakeExecProvider() FakeExecProvider {
	return FakeExecProvider{
		findOnPathFunc: func(file string) (string, error) {
			return "/fake/path/to/kubectl/binary", nil
		},
		execCmdFunc: func(name string, arg ...string) ([]byte, error) {
			return nil, nil
		},
	}
}

func TestFakeKubectlCommandFoundInPath(t *testing.T) {
	restoreExecProvider := execProvider
	defer func() { execProvider = restoreExecProvider }()

	execProvider = BaseFakeExecProvider()

	logger := testlogr.TestLogger{T: t}
	k, err := NewKubectl(logger)
	t.Logf("Kubectl (%T) %#v", k, k)
	if err != nil {
		t.Errorf("Error returned from the execLookPath function : %w", err)
	}
	t.Logf("Kubectl command path '%s'", k.getPath())
}

func TestFakeKubectlCommandNotFoundInPath(t *testing.T) {
	restoreExecProvider := execProvider
	defer func() { execProvider = restoreExecProvider }()

	execProvider = BaseFakeExecProvider().setFindOnPathFunc(
		func(file string) (string, error) {
			return "", fmt.Errorf("exec \"%s\": executable file not found in $PATH", file)
		},
	)

	logger := testlogr.TestLogger{T: t}
	k, err := NewKubectl(logger)
	t.Logf("Kubectl (%T) %#v", k, k)
	if err == nil {
		t.Errorf("Expected error 'executable file not found' was not returned from NewKubectl constructor")
	} else {
		t.Logf("Expected error was returned: %#v", err)
	}
}

func TestKubectlCommandFoundInPath(t *testing.T) {
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	restoreExecProvider := execProvider
	defer func() { execProvider = restoreExecProvider }()
	mockExecProvider := NewMockExecProvider(mockCtl)
	execProvider = mockExecProvider

	// Verifies that the "findOnPath" method was called once with the exact expected string input as the parameter
	mockExecProvider.EXPECT().findOnPath(kubectlCmd).Return("/mocked/path/to/kubectl/binary", nil).Times(1)

	logger := testlogr.TestLogger{T: t}
	k, err := NewKubectl(logger)
	t.Logf("Kubectl (%T) %#v", k, k)
	if err != nil {
		t.Errorf("Error returned from the execLookPath function : %w", err)
	} else {
		t.Logf("Kubectl command path '%s'", k.getPath())
	}
}

func TestKubectlCommandNotFoundInPath(t *testing.T) {
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	restoreExecProvider := execProvider
	defer func() { execProvider = restoreExecProvider }()

	mockExecProvider := NewMockExecProvider(mockCtl)
	execProvider = mockExecProvider

	// Verifies that the "findOnPath" method was called once with the exact expected string input as the parameter
	mockExecProvider.EXPECT().findOnPath(kubectlCmd).Return("MOCKED", fmt.Errorf("MOCK exec \"%s\": executable file not found in $PATH", kubectlCmd)).Times(1)

	logger := testlogr.TestLogger{T: t}
	k, err := NewKubectl(logger)
	t.Logf("Kubectl (%T) %#v", k, k)
	if err == nil {
		t.Errorf("Expected error 'executable file not found' was not returned from NewKubectl constructor")
	} else {
		t.Logf("Expected error was returned: %#v", err)
	}
}

func TestKubectlApplyReturnsApplyCommand(t *testing.T) { // nolint:funlen,gocyclo // allow longer test functions
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	restoreExecProvider := execProvider
	defer func() { execProvider = restoreExecProvider }()

	mockExecProvider := NewMockExecProvider(mockCtl)
	execProvider = mockExecProvider

	fakeCommandPath := "/mocked/path/to/kubectl/binary"
	// Verifies that the "findOnPath" method was called once with the exact expected string input as the parameter
	mockExecProvider.EXPECT().findOnPath(kubectlCmd).Return(fakeCommandPath, nil).Times(1)

	logger := testlogr.TestLogger{T: t}
	k, err := NewKubectl(logger)
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
	a, ok := c.(*ApplyCommand)
	if !ok {
		t.Logf("error casting to ApplyCommand")
	}

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

func TestKubectlApplyRunHandlesExecError(t *testing.T) {
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
	expectedOutput := "FAKE OUTPUT"
	// Verifies that the "findOnPath" method was called once with the exact expected string input as the parameter
	mockExecProvider.EXPECT().findOnPath(kubectlCmd).Return(fakeCommandPath, nil).Times(1)
	mockExecProvider.EXPECT().execCmd(fakeCommandPath, expectedArgs).Return([]byte(expectedOutput), nil).Times(1)

	logger := testlogr.TestLogger{T: t}
	k, err := NewKubectl(logger)
	t.Logf("Kubectl (%T) %#v", k, k)
	if err != nil {
		t.Errorf("Error returned from the execLookPath function : %w", err)
	}

	c := k.Apply(fakeSourceDir)
	if c == nil {
		t.Fatalf("nil returned from Kubectl.Apply(sourcedir) function")
	}
	t.Logf("Returned (%T) %#v", c, c)

	gotOutput, err := c.Run()
	if err != nil {
		t.Errorf("Error returned from ApplyCommand.Run: %w", err)
		t.Fatalf("Error returned from ApplyCommand.Run")
	}

	if !bytes.Equal([]byte(expectedOutput), gotOutput) {
		t.Fatalf("expected output '%s', got output '%s' from ApplyCommand.Run", expectedOutput, string(gotOutput))
	} else {
		t.Logf("ApplyCommand.Run expected output '%s' matches output '%s'", expectedOutput, string(gotOutput))
	}
}

func TestKubectlApplyDryRun(t *testing.T) {
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	restoreExecProvider := execProvider
	defer func() { execProvider = restoreExecProvider }()

	mockExecProvider := NewMockExecProvider(mockCtl)
	execProvider = mockExecProvider

	fakeCommandPath := "/mocked/path/to/kubectl"
	expectedSubCmd := "apply"
	fakeSourceDir := "/mocked/path/to/source/directory"
	expectedArgs := []string{expectedSubCmd, "-R", "-f", fakeSourceDir, "--dry-run", "-o", "json"}
	expectedOutput := "FAKE OUTPUT"
	// Verifies that the "findOnPath" method was called once with the exact expected string input as the parameter
	mockExecProvider.EXPECT().findOnPath(kubectlCmd).Return(fakeCommandPath, nil).Times(1)
	mockExecProvider.EXPECT().execCmd(fakeCommandPath, expectedArgs).Return([]byte(expectedOutput), nil).Times(1)

	logger := testlogr.TestLogger{T: t}
	k, err := NewKubectl(logger)
	t.Logf("Kubectl (%T) %#v", k, k)
	if err != nil {
		t.Errorf("Error returned from the execLookPath function : %w", err)
	}

	c := k.Apply(fakeSourceDir)
	if c == nil {
		t.Fatalf("nil returned from Kubectl.Apply(sourcedir) function")
	}
	t.Logf("Returned (%T) %#v", c, c)

	gotOutput, err := c.DryRun()
	if err != nil {
		t.Errorf("Error returned from ApplyCommand.Run: %w", err)
		t.Fatalf("Error returned from ApplyCommand.Run")
	}

	if !bytes.Equal([]byte(expectedOutput), gotOutput) {
		t.Fatalf("expected output '%s', got output '%s' from ApplyCommand.Run", expectedOutput, string(gotOutput))
	} else {
		t.Logf("ApplyCommand.Run expected output '%s' matches output '%s'", expectedOutput, string(gotOutput))
	}
}

func TestKubectlApplyRun(t *testing.T) {
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

	logger := testlogr.TestLogger{T: t}
	k, err := NewKubectl(logger)
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

func TestKubectlCommandUsesKubectlLogger(t *testing.T) {
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
	expectedCmd := fmt.Sprintf("%s %s", fakeCommandPath, strings.Join(expectedArgs, " "))
	expectedOutput := "FAKE OUTPUT"
	// Verifies that the "findOnPath" method was called once with the exact expected string input as the parameter
	mockExecProvider.EXPECT().findOnPath(kubectlCmd).Return(fakeCommandPath, nil).AnyTimes()
	mockExecProvider.EXPECT().execCmd(fakeCommandPath, expectedArgs).Return([]byte(expectedOutput), nil).AnyTimes()

	mockLogger := mocklogr.NewMockLogger(mockCtl)

	// The ApplyCommand should use the mockLogger to log a log-leveled message indicating that kubectl was executed with the expected command
	mockLogger.EXPECT().V(gomock.Any()).Return(mockLogger).Times(1)
	mockLogger.EXPECT().Info("executing kubectl", "command", expectedCmd).Times(1)

	// Create the Kubectl type instance with the wrongLogger
	k, err := NewKubectl(mockLogger)
	t.Logf("Kubectl (%T) %#v", k, k)
	if err != nil {
		t.Errorf("Error returned from the execLookPath function : %w", err)
	}

	// Create the ApplyCommand from the Kubectl factory
	output, err := k.Apply(fakeSourceDir).Run()
	if err != nil {
		t.Fatalf("nil returned from Kubectl.Apply(sourcedir) function")
	}
	t.Logf("output: (%T) %#v", output, output)
}

func TestKubectlCommandWithLoggerPassedNil(t *testing.T) {
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
	expectedCmd := fmt.Sprintf("%s %s", fakeCommandPath, strings.Join(expectedArgs, " "))
	expectedOutput := "FAKE OUTPUT"
	// Verifies that the "findOnPath" method was called once with the exact expected string input as the parameter
	mockExecProvider.EXPECT().findOnPath(kubectlCmd).Return(fakeCommandPath, nil).AnyTimes()
	mockExecProvider.EXPECT().execCmd(fakeCommandPath, expectedArgs).Return([]byte(expectedOutput), nil).AnyTimes()

	mockLogger := mocklogr.NewMockLogger(mockCtl)

	// The ApplyCommand should use the mockLogger to log a log-leveled message indicating that kubectl was executed with the expected command
	mockLogger.EXPECT().V(gomock.Any()).Return(mockLogger).Times(1)
	mockLogger.EXPECT().Info("executing kubectl", "command", expectedCmd).Times(1)

	// Create the Kubectl type instance with the wrongLogger
	k, err := NewKubectl(mockLogger)
	t.Logf("Kubectl (%T) %#v", k, k)
	if err != nil {
		t.Errorf("Error returned from the execLookPath function : %w", err)
	}

	// Create the ApplyCommand from the Kubectl factory and try passing nil to Withlogger
	output, err := k.Apply(fakeSourceDir).WithLogger(nil).Run()
	if err != nil {
		t.Fatalf("nil returned from Kubectl.Apply(sourcedir) function")
	}
	t.Logf("output: (%T) %#v", output, output)
}

func TestKubectlCommandWithLoggerChangesTheCommandLogger(t *testing.T) {
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
	expectedCmd := fmt.Sprintf("%s %s", fakeCommandPath, strings.Join(expectedArgs, " "))
	expectedOutput := "FAKE OUTPUT"
	// Verifies that the "findOnPath" method was called once with the exact expected string input as the parameter
	mockExecProvider.EXPECT().findOnPath(kubectlCmd).Return(fakeCommandPath, nil).AnyTimes()
	mockExecProvider.EXPECT().execCmd(fakeCommandPath, expectedArgs).Return([]byte(expectedOutput), nil).AnyTimes()

	wrongLogger := mocklogr.NewMockLogger(mockCtl)
	rightLogger := mocklogr.NewMockLogger(mockCtl)

	// The wrongLogger should never be used
	wrongLogger.EXPECT().V(gomock.Any()).Return(wrongLogger).Times(0)
	wrongLogger.EXPECT().Info(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	// The rightLogger should be used to log a log-leveled message indicating that kubectl was executed with the expected command
	rightLogger.EXPECT().V(gomock.Any()).Return(rightLogger).Times(1)
	rightLogger.EXPECT().Info("executing kubectl", "command", expectedCmd).Times(1)

	// Create the Kubectl type instance with the wrongLogger
	k, err := NewKubectl(wrongLogger)
	t.Logf("Kubectl (%T) %#v", k, k)
	if err != nil {
		t.Errorf("Error returned from the execLookPath function : %w", err)
	}

	// Create the ApplyCommand from the Kubectl factory, and swap in the rightLogger with the WithLogger function
	output, err := k.Apply(fakeSourceDir).WithLogger(rightLogger).Run()
	if err != nil {
		t.Fatalf("nil returned from Kubectl.Apply(sourcedir) function")
	}
	t.Logf("output: (%T) %#v", output, output)
}
