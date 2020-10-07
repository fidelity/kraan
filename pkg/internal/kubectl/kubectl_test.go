package kubectl_test

/*

The mockgen tool generates the MockExecProvider type defined in the kubectl/mockExecProvider.go code file.

From the project root directory, you can generate mock definitions for interfaces in individual code files by calling mockgen.  Example:
	mockgen -destination=pkg/internal/kubectl/mockExecProvider.go -package=kubectl -source=pkg/internal/kubectl/execProvider.go \
	github.com/fidelity/kraan/pkg/internal/kubectl ExecProvider

Or you can allow `go generate` to create all mocks for a project or package in a single command.

Add a go:generate annotation above the package statement in all the code files containing interfaces that you want to mock.  Example:
	//go:generate mockgen -destination=mockExecProvider.go -package=kubectl -source=execProvider.go . ExecProvider
	//go:generate mockgen -destination=../mocks/logr/mockLogger.go -package=mocks github.com/go-logr/logr Logger

From the project root directory, you can then generate mocks for all the interfaces that have a go:generate annotation by running 'go generate ./...'.

*/
import (
	"bytes"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	testlogr "github.com/go-logr/logr/testing"
	gomock "github.com/golang/mock/gomock"

	"github.com/fidelity/kraan/pkg/internal/kubectl"
	mocks "github.com/fidelity/kraan/pkg/internal/mocks/kubectl"
	mocklogr "github.com/fidelity/kraan/pkg/internal/mocks/logr"
)

var (
	errFromExec         = fmt.Errorf("error from executable")
	errExecFileNotFound = fmt.Errorf("executable file not found in $PATH")
	kubectlPath         = "/mocked/path/to/kubectl"
	sourcePath          = "/mocked/path/to/source/directory"
	kustomizeFile       = "kustomization.yaml"
)

func TestNewKubectl(t *testing.T) {
	s := setupApply(t).expectKubectlFound()
	logger := testlogr.TestLogger{T: t}
	k, err := kubectl.NewKubectl(logger)
	if err != nil {
		t.Errorf("The NewKubectl function returned an error! %w", err)
	}
	t.Logf("k (%T) %#v", k, k)
	fType := &kubectl.CommandFactory{}
	if reflect.TypeOf(k) != reflect.TypeOf(fType) {
		t.Fatalf("The NewKubectl function did not return an instance of %T", fType)
	}
	f, ok := k.(*kubectl.CommandFactory)
	if !ok {
		t.Fatalf("Failed to cast Kubectl instance to %T : %#v", fType, k)
	}
	gotLogger := kubectl.GetLogger(*f)
	t.Logf("gotLogger (%T) %#v", gotLogger, gotLogger)
	if logger != gotLogger {
		t.Errorf("The passed logger was not stored as the new Kubectl instance's Logger")
	}
	gotExecProvider := kubectl.GetExecProvider(*f)
	if gotExecProvider == nil {
		t.Errorf("The NewKubectl function did not instantiate an execProvider!")
	}
	if gotExecProvider != s.execProvider {
		t.Errorf("The NewKubectl function did not call the function referenced by the newExecProviderFunc package var")
	}
	t.Logf("kubectl full path: %s", kubectl.GetFactoryPath(*f))
}

func TestKubectlCommandFoundInPath(t *testing.T) {
	s := setupApply(t).expectKubectlFound()
	defer s.restoreFunc()

	f, err := kubectl.NewCommandFactory(s.testLogr, s.execProvider)
	t.Logf("Kubectl (%T) %#v", f, f)
	if err != nil {
		t.Errorf("Error returned from the execLookPath function : %w", err)
	} else {
		t.Logf("Kubectl command path '%s'", kubectl.GetFactoryPath(*f))
	}
}

func TestKubectlCommandNotFoundInPath(t *testing.T) {
	s := setupApply(t).expectKubectlNotFound()
	defer s.restoreFunc()

	f, err := kubectl.NewCommandFactory(s.testLogr, s.execProvider)
	t.Logf("Kubectl (%T) %#v", f, f)
	if err == nil {
		t.Errorf("Expected error 'executable file not found' was not returned from NewKubectl constructor")
	} else {
		t.Logf("Expected error was returned: %#v", err)
	}
}

func TestKubectlApplyReturnsApplyCommand(t *testing.T) {
	s := setupApply(t).expectKubectlFound().withFactory()
	defer s.restoreFunc()
	s.validateCommandState(s.Apply())
}

func TestKubectlApplyChecksForKustomizeFile(t *testing.T) {
	s := setupApply(t).WithKustomize().expectKubectlFound().withFactory()
	defer s.restoreFunc()
	s.validateCommandState(s.Apply())
}

func TestGetReturnsGetCommand(t *testing.T) {
	s := setupGet(t).expectKubectlFound().withFactory()
	defer s.restoreFunc()
	s.validateCommandState(s.Get())
}

func TestDeleteReturnsDeleteCommand(t *testing.T) {
	s := setupDelete(t).expectKubectlFound().withFactory()
	defer s.restoreFunc()
	s.validateCommandState(s.Delete())
}

func TestKubectlApplyRunReturnsOutput(t *testing.T) {
	s := setupApply(t).expectRun().withFactory()
	defer s.restoreFunc()

	gotOutput, err := s.Apply().Run()
	if err != nil {
		t.Errorf("Error returned from ApplyCommand.Run: %w", err)
		t.Fatalf("Error returned from ApplyCommand.Run")
	}

	if !bytes.Equal([]byte(s.output), gotOutput) {
		t.Fatalf("expected output '%s', got output '%s' from ApplyCommand.Run", s.output, string(gotOutput))
	} else {
		t.Logf("ApplyCommand.Run expected output '%s' matches output '%s'", s.output, string(gotOutput))
	}
}

func TestKubectlApplyDryRunReturnsOutput(t *testing.T) {
	s := setupApply(t).expectDryRun().withFactory()
	defer s.restoreFunc()

	gotOutput, err := s.Apply().DryRun()
	if err != nil {
		t.Errorf("Error returned from ApplyCommand.DryRun: %w", err)
		t.Fatalf("Error returned from ApplyCommand.DryRun")
	}

	if !bytes.Equal([]byte(s.output), gotOutput) {
		t.Fatalf("expected output '%s', got output '%s' from ApplyCommand.Run", s.output, string(gotOutput))
	} else {
		t.Logf("ApplyCommand.Run expected output '%s' matches output '%s'", s.output, string(gotOutput))
	}
}

func TestKubectlApplyRunReturnsExecError(t *testing.T) {
	s := setupApply(t).expectRunError().withFactory()
	defer s.restoreFunc()

	_, err := s.Apply().Run()
	if err == nil {
		t.Fatalf("Expected error was not returned from ApplyCommand.Run!")
	}
}

func TestKubectlRunUsesFactoryLogger(t *testing.T) {
	s := setupApply(t).expectRunLogsKubectlCommand().withFactoryMockLogr()
	defer s.restoreFunc()

	_, err := s.Apply().Run()
	if err != nil {
		t.Errorf("Error returned from ApplyCommand.DryRun: %w", err)
		t.Fatalf("Error returned from ApplyCommand.DryRun")
	}
}

func TestKubectlCommandWithLoggerPassedNil(t *testing.T) {
	s := setupApply(t).expectRunLogsKubectlCommand().withFactoryMockLogr()
	defer s.restoreFunc()

	// Create the ApplyCommand from the Kubectl factory and try passing nil to Withlogger
	_, err := s.Apply().WithLogger(nil).Run()
	// No error, and the message should be logged to the CommandFactory's logger as expected
	if err != nil {
		t.Errorf("Error returned from ApplyCommand.Run: %w", err)
		t.Fatalf("Error returned from ApplyCommand.Run")
	}
}

func TestKubectlApplyRunLogsKubectlCommand(t *testing.T) {
	s := setupApply(t).expectRunLogsKubectlCommand().withFactoryMockLogr()
	defer s.restoreFunc()

	_, err := s.Apply().Run()
	if err != nil {
		t.Errorf("Error returned from ApplyCommand.Run: %w", err)
		t.Fatalf("Error returned from ApplyCommand.Run")
	}
}

func TestKubectlApplyDryRunLogsKubectlCommand(t *testing.T) {
	s := setupApply(t).expectDryRunLogsKubectlCommand().withFactoryMockLogr()
	defer s.restoreFunc()

	_, err := s.Apply().DryRun()
	if err != nil {
		t.Errorf("Error returned from ApplyCommand.Run: %w", err)
		t.Fatalf("Error returned from ApplyCommand.Run")
	}
}

func TestKubectlApplyRunSetsKustomizeFlag(t *testing.T) {
	s := setupApply(t).WithKustomize().expectRunLogsKubectlCommand().withFactoryMockLogr()
	defer s.restoreFunc()

	_, err := s.Apply().Run()
	if err != nil {
		t.Errorf("Error returned from ApplyCommand.Run: %w", err)
		t.Fatalf("Error returned from ApplyCommand.Run")
	}
}

func TestKubectlCommandWithLoggerUsesPassedLogger(t *testing.T) {
	s := setupApply(t).expectRunUsesCommandLogger().withFactoryMockLogr()
	defer s.restoreFunc()

	// Create the ApplyCommand from the Kubectl factory, and swap in the rightLogger with the WithLogger function
	_, err := s.Apply().WithLogger(s.cmdLogr).Run()
	if err != nil {
		t.Errorf("Error returned from ApplyCommand.Run: %w", err)
		t.Fatalf("Error returned from ApplyCommand.Run")
	}
}

type Setup struct {
	t             *testing.T
	mockCtl       *gomock.Controller
	execProvider  *mocks.MockExecProvider
	factory       *kubectl.CommandFactory
	path          string
	subCmd        string
	sourceDir     string
	withKustomize bool
	args          []string
	cmdArgs       []string
	runArgs       []string
	dryRunArgs    []string
	cmdStr        string
	runStr        string
	dryRunStr     string
	expectJSON    bool
	output        string
	testLogr      testlogr.TestLogger
	mockLogr      *mocklogr.MockLogger
	cmdLogr       *mocklogr.MockLogger
	restoreFunc   func()
}

func setup(t *testing.T, subCmd string, expectJSON bool) *Setup {
	mockCtl := gomock.NewController(t)

	mockExecProvider := mocks.NewMockExecProvider(mockCtl)

	kubectl.SetNewExecProviderFunc(func() kubectl.ExecProvider { return mockExecProvider })

	restoreFunc := func() {
		mockCtl.Finish()
	}

	return &Setup{
		t:             t,
		mockCtl:       mockCtl,
		execProvider:  mockExecProvider,
		subCmd:        subCmd,
		sourceDir:     sourcePath,
		withKustomize: false,
		path:          kubectlPath,
		expectJSON:    expectJSON,
		output:        "FAKE OUTPUT",
		testLogr:      testlogr.TestLogger{T: t},
		mockLogr:      mocklogr.NewMockLogger(mockCtl),
		cmdLogr:       mocklogr.NewMockLogger(mockCtl),
		restoreFunc:   restoreFunc,
	}
}

func setupApply(t *testing.T) *Setup {
	return setup(t, "apply", true).WithArgs("-R", "-f", sourcePath)
}

func setupDelete(t *testing.T) *Setup {
	return setup(t, "delete", false).WithArgs("pod", "random-pod", "-n", "some-namespace")
}

func setupGet(t *testing.T) *Setup {
	return setup(t, "get", true).WithArgs("helmrelease", "addon", "-n", "kube-system")
}

func (s *Setup) WithArgs(expectedArgs ...string) *Setup {
	s.args = expectedArgs
	s.cmdArgs = append([]string{s.subCmd}, expectedArgs...)
	s.runArgs = s.cmdArgs
	s.dryRunArgs = append(s.runArgs, "--server-dry-run")
	if s.expectJSON {
		jsonArgs := []string{"-o", "json"}
		s.runArgs = append(s.runArgs, jsonArgs...)
		s.dryRunArgs = append(s.dryRunArgs, jsonArgs...)
	}
	s.cmdStr = fmt.Sprintf("%s %s", kubectlPath, strings.Join(s.cmdArgs, " "))
	s.runStr = fmt.Sprintf("%s %s", kubectlPath, strings.Join(s.runArgs, " "))
	s.dryRunStr = fmt.Sprintf("%s %s", kubectlPath, strings.Join(s.dryRunArgs, " "))
	return s
}

func (s *Setup) WithKustomize() *Setup {
	s.withKustomize = true
	return s.WithArgs("-k", s.sourceDir)
}

func (s *Setup) expectKustomize() *Setup {
	kustomizePath := filepath.Join(sourcePath, kustomizeFile)
	s.execProvider.EXPECT().FileExists(kustomizePath).Return(s.withKustomize).Times(1)
	return s
}

func (s *Setup) expectKubectlFound() *Setup {
	s.execProvider.EXPECT().FindOnPath(kubectl.KubectlCmd).Return(s.path, nil).Times(1)
	return s
}

func (s *Setup) expectKubectlNotFound() *Setup {
	s.execProvider.EXPECT().FindOnPath(kubectl.KubectlCmd).Return(s.path, errExecFileNotFound).Times(1)
	return s
}

func (s *Setup) withFactoryLogr(logger logr.Logger) *Setup {
	factory, err := kubectl.NewCommandFactory(logger, s.execProvider)
	s.t.Logf("Kubectl (%T) %#v", factory, factory)
	if err != nil {
		s.t.Errorf("Error returned from the execLookPath function : %w", err)
	}
	s.factory = factory
	return s
}

func (s *Setup) withFactory() *Setup {
	return s.withFactoryLogr(s.testLogr)
}

func (s *Setup) withFactoryMockLogr() *Setup {
	return s.withFactoryLogr(s.mockLogr)
}

func (s *Setup) expectRun() *Setup {
	s.expectKubectlFound()
	s.execProvider.EXPECT().ExecCmd(s.path, s.runArgs).Return([]byte(s.output), nil).Times(1)
	return s
}

func (s *Setup) expectDryRun() *Setup {
	s.expectKubectlFound()
	s.execProvider.EXPECT().ExecCmd(s.path, s.dryRunArgs).Return([]byte(s.output), nil).Times(1)
	return s
}

func (s *Setup) expectRunError() *Setup {
	s.expectKubectlFound()
	s.execProvider.EXPECT().ExecCmd(s.path, s.runArgs).Return(nil, errFromExec).Times(1)
	return s
}

func (s *Setup) expectRunLogsKubectlCommand() *Setup {
	s.expectRun()
	// The ApplyCommand should use the factory's mockLogger to log a log-leveled message indicating that kubectl was executed with the expected command
	s.mockLogr.EXPECT().V(gomock.Any()).Return(s.mockLogr).Times(1)
	s.mockLogr.EXPECT().Info("executing kubectl", "command", s.runStr).Times(1)
	return s
}

func (s *Setup) expectDryRunLogsKubectlCommand() *Setup {
	s.expectDryRun()
	// The ApplyCommand should use the factory's mockLogger to log a log-leveled message indicating that kubectl was executed with the expected command
	s.mockLogr.EXPECT().V(gomock.Any()).Return(s.mockLogr).Times(1)
	s.mockLogr.EXPECT().Info("executing kubectl", "command", s.dryRunStr).Times(1)
	return s
}

func (s *Setup) expectRunUsesCommandLogger() *Setup {
	s.expectRun()
	// The ApplyCommand should use the command's mockLogger to log a log-leveled message indicating that kubectl was executed with the expected command
	s.cmdLogr.EXPECT().V(gomock.Any()).Return(s.cmdLogr).Times(1)
	s.cmdLogr.EXPECT().Info("executing kubectl", "command", s.runStr).Times(1)
	// The Factory's mockLogger should never be used
	s.mockLogr.EXPECT().V(gomock.Any()).Return(s.mockLogr).Times(0)
	s.mockLogr.EXPECT().Info(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	return s
}

func (s *Setup) validateCommand(command kubectl.Command, cmdType kubectl.Command, functionName string) {
	if command == nil {
		s.t.Fatalf("nil returned from Kubectl %s function", functionName)
	}
	s.t.Logf("%s Returned (%T) %#v for (%T) %#v", functionName, command, command, cmdType, cmdType)
	if reflect.TypeOf(command) == reflect.TypeOf(cmdType) {
		s.t.Logf("Expected type (%T) match for %#v", cmdType, command)
	} else {
		s.t.Logf("return value from Kubectl.%s is not of type %T: %#v", functionName, cmdType, command)
	}
}

func (s *Setup) validateCommandState(c kubectl.Command) {
	typ := reflect.TypeOf(c).String()

	if s.subCmd != kubectl.GetSubCmd(c) {
		s.t.Fatalf("expected '%s', got '%s' for the returned %s.GetSubCmd", s.subCmd, kubectl.GetSubCmd(c), typ)
	} else {
		s.t.Logf("SubCmd expected '%s' matches '%s'", s.subCmd, kubectl.GetSubCmd(c))
	}

	if len(kubectl.GetArgs(c)) != len(s.cmdArgs) {
		s.t.Errorf("expected %d args, got %d args in the returned %s", len(s.cmdArgs), len(kubectl.GetArgs(c)), typ)
	}
	argsEqual := true
	for i, arg := range kubectl.GetArgs(c) {
		expectedArg := s.cmdArgs[i]
		if arg != expectedArg {
			s.t.Errorf("expected arg '%s', got arg '%s' at index [%d] in the returned %s", expectedArg, arg, i, typ)
			argsEqual = false
		} else {
			s.t.Logf("at index [%d] expected arg '%s' matches arg '%s'", i, expectedArg, arg)
		}
	}
	if !argsEqual {
		s.t.Fatalf("args in the returned %s did not match expectations", typ)
	}

	if s.path != kubectl.GetCommandPath(c) {
		s.t.Fatalf("expected '%s', got '%s' for Path in the returned %s", s.path, kubectl.GetCommandPath(c), typ)
	} else {
		s.t.Logf("Path expected '%s' matches '%s'", s.path, kubectl.GetCommandPath(c))
	}

	if s.cmdStr != kubectl.AsCommandString(c) {
		s.t.Fatalf("expected '%s', got '%s' from the returned %s AsString function", s.cmdStr, kubectl.AsCommandString(c), typ)
	} else {
		s.t.Logf("AsString expected '%s' matches '%s'", s.cmdStr, kubectl.AsCommandString(c))
	}

	if s.expectJSON != kubectl.IsJSONOutput(c) {
		s.t.Fatalf("expected %v, got %v from the returned %s IsJSONOutput function", s.expectJSON, kubectl.IsJSONOutput(c), typ)
	}
}

func (s *Setup) Apply() *kubectl.ApplyCommand {
	command := s.expectKustomize().factory.Apply(s.sourceDir)
	s.validateCommand(command, &kubectl.ApplyCommand{}, "Apply")
	typedCommand, ok := command.(*kubectl.ApplyCommand)
	if !ok {
		s.t.Logf("error casting to *kubectl.ApplyCommand! %#v", command)
	}
	return typedCommand
}

func (s *Setup) Get() *kubectl.GetCommand {
	command := s.factory.Get(s.args...)
	s.validateCommand(command, &kubectl.GetCommand{}, "Get")
	typedCommand, ok := command.(*kubectl.GetCommand)
	if !ok {
		s.t.Logf("error casting to *kubectl.GetCommand! %#v", command)
	}
	return typedCommand
}

func (s *Setup) Delete() *kubectl.DeleteCommand {
	command := s.factory.Delete(s.args...)
	s.validateCommand(command, &kubectl.DeleteCommand{}, "Delete")
	typedCommand, ok := command.(*kubectl.DeleteCommand)
	if !ok {
		s.t.Logf("error casting to *kubectl.DeleteCommand! %#v", command)
	}
	return typedCommand
}
