package kubectl_test

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
	kustomizePath       = "/mocked/path/to/kustomize"
	sourcePath          = "/mocked/path/to/source/directory"
	kustomizeFile       = "kustomization.yaml"
	tempDir             = "/tmp/temp-dir"
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

func TestNewKustomize(t *testing.T) {
	s := setupApply(t).expectKustomizeFound()
	logger := testlogr.TestLogger{T: t}
	k, err := kubectl.NewKustomize(logger)
	if err != nil {
		t.Errorf("The NewKustomize function returned an error! %w", err)
	}
	t.Logf("k (%T) %#v", k, k)
	fType := &kubectl.CommandFactory{}
	if reflect.TypeOf(k) != reflect.TypeOf(fType) {
		t.Fatalf("The NewKustomize function did not return an instance of %T", fType)
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
		t.Errorf("The NewKustomize function did not instantiate an execProvider!")
	}
	if gotExecProvider != s.execProvider {
		t.Errorf("The NewKustomize function did not call the function referenced by the newExecProviderFunc package var")
	}
	t.Logf("kubectl full path: %s", kubectl.GetFactoryPath(*f))
}

func TestKubectlCommandFoundInPath(t *testing.T) {
	s := setupApply(t).withKustomize().expectKubectlFound()
	defer s.restoreFunc()

	f, err := kubectl.NewCommandFactory(s.testLogr, s.execProvider, kubectl.KubectlCmd)
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

	f, err := kubectl.NewCommandFactory(s.testLogr, s.execProvider, kubectl.KubectlCmd)
	t.Logf("Kubectl (%T) %#v", f, f)
	if err == nil {
		t.Errorf("Expected error 'executable file not found' was not returned from NewKubectl constructor")
	} else {
		t.Logf("Expected error was returned: %#v", err)
	}
}

func TestKustomizeBuildReturnsBuildCommand(t *testing.T) {
	s := setupBuild(t).expectKustomizeFound().withKustomizeFactory()
	defer s.restoreFunc()
	s.validateCommandState(s.Build())
}

func TestKubectlApplyReturnsApplyCommand(t *testing.T) {
	s := setupApply(t).expectNoKustomize().expectKubectlFound().withFactory()
	defer s.restoreFunc()
	s.validateCommandState(s.Apply())
}

func TestKubectlApplyChecksForKustomizeFile(t *testing.T) {
	s := setupApply(t).expectNoKustomize().expectKubectlFound().withFactory()
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

func TestKustomizeBuild(t *testing.T) {
	s := setupBuild(t).expectBuildRun([]string{"-o", tempDir}).withKustomizeFactory()
	defer s.restoreFunc()

	path := s.Build().Build()
	if path == "" {
		t.Fatalf("expected a path")
	}
}

func TestKubectlApplyRunReturnsOutput(t *testing.T) {
	s := setupApply(t).expectNoKustomize().expectRun().withFactory()
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
	s := setupApply(t).expectNoKustomize().expectDryRun().withFactory()
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
	s := setupApply(t).expectNoKustomize().expectRunError().withFactory()
	defer s.restoreFunc()

	_, err := s.Apply().Run()
	if err == nil {
		t.Fatalf("Expected error was not returned from ApplyCommand.Run!")
	}
}

func TestKubectlRunUsesFactoryLogger(t *testing.T) {
	s := setupApply(t).expectNoKustomize().expectRunLogsKubectlCommand().withFactoryMockLogr()
	defer s.restoreFunc()

	_, err := s.Apply().Run()
	if err != nil {
		t.Errorf("Error returned from ApplyCommand.DryRun: %w", err)
		t.Fatalf("Error returned from ApplyCommand.DryRun")
	}
}

func TestKubectlCommandWithLoggerPassedNil(t *testing.T) {
	s := setupApply(t).expectNoKustomize().expectRunLogsKubectlCommand().withFactoryMockLogr()
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
	s := setupApply(t).expectNoKustomize().expectRunLogsKubectlCommand().withFactoryMockLogr()
	defer s.restoreFunc()

	_, err := s.Apply().Run()
	if err != nil {
		t.Errorf("Error returned from ApplyCommand.Run: %w", err)
		t.Fatalf("Error returned from ApplyCommand.Run")
	}
}

func TestKubectlApplyDryRunLogsKubectlCommand(t *testing.T) {
	s := setupApply(t).expectNoKustomize().expectDryRunLogsKubectlCommand().withFactoryMockLogr()
	defer s.restoreFunc()

	_, err := s.Apply().DryRun()
	if err != nil {
		t.Errorf("Error returned from ApplyCommand.Run: %w", err)
		t.Fatalf("Error returned from ApplyCommand.Run")
	}
}

func TestKubectlCommandWithLoggerUsesPassedLogger(t *testing.T) {
	s := setupApply(t).expectNoKustomize().expectRunUsesCommandLogger().withFactoryMockLogr()
	defer s.restoreFunc()

	// Create the ApplyCommand from the Kubectl factory, and swap in the rightLogger with the WithLogger function
	_, err := s.Apply().WithLogger(s.cmdLogr).Run()
	if err != nil {
		t.Errorf("Error returned from ApplyCommand.Run: %w", err)
		t.Fatalf("Error returned from ApplyCommand.Run")
	}
}

type Setup struct {
	t            *testing.T
	mockCtl      *gomock.Controller
	execProvider *mocks.MockExecProvider
	factory      *kubectl.CommandFactory
	path         string
	subCmd       string
	sourceDir    string
	applyDir     string
	args         []string
	cmdArgs      []string
	runArgs      []string
	dryRunArgs   []string
	cmdStr       string
	runStr       string
	dryRunStr    string
	expectJSON   bool
	output       string
	testLogr     testlogr.TestLogger
	mockLogr     *mocklogr.MockLogger
	cmdLogr      *mocklogr.MockLogger
	restoreFunc  func()
}

func setup(t *testing.T, subCmd string, expectJSON bool) *Setup {
	mockCtl := gomock.NewController(t)

	mockExecProvider := mocks.NewMockExecProvider(mockCtl)

	kubectl.SetNewExecProviderFunc(func() kubectl.ExecProvider { return mockExecProvider })
	kubectl.SetNewTempDirProviderFunc(func() (string, error) { return tempDir, nil })
	restoreFunc := func() {
		mockCtl.Finish()
	}

	return &Setup{
		t:            t,
		mockCtl:      mockCtl,
		execProvider: mockExecProvider,
		subCmd:       subCmd,
		sourceDir:    sourcePath,
		applyDir:     sourcePath,
		path:         kubectlPath,
		expectJSON:   expectJSON,
		output:       "FAKE OUTPUT",
		testLogr:     testlogr.TestLogger{T: t},
		mockLogr:     mocklogr.NewMockLogger(mockCtl),
		cmdLogr:      mocklogr.NewMockLogger(mockCtl),
		restoreFunc:  restoreFunc,
	}
}

func setupApply(t *testing.T) *Setup {
	return setup(t, "apply", true).WithArgs("-R", "-f", sourcePath)
}

func setupBuild(t *testing.T) *Setup {
	s := setup(t, "build", true).WithKustomizeArgs(sourcePath)
	s.path = "/mocked/path/to/kustomize"
	return s
}

func setupDelete(t *testing.T) *Setup {
	return setup(t, "delete", false).WithArgs("pod", "random-pod", "-n", "some-namespace")
}

func setupGet(t *testing.T) *Setup {
	return setup(t, "get", true).WithArgs("helmrelease", "addon", "-n", "kube-system")
}

func (s *Setup) WithKustomizeArgs(expectedArgs ...string) *Setup {
	s.args = expectedArgs
	s.cmdArgs = append([]string{s.subCmd}, expectedArgs...)
	s.runArgs = s.cmdArgs
	s.cmdStr = fmt.Sprintf("%s %s", kustomizePath, strings.Join(s.cmdArgs, " "))
	s.runStr = fmt.Sprintf("%s %s", kustomizePath, strings.Join(s.runArgs, " "))
	return s
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

func (s *Setup) withKustomize() *Setup {
	s.applyDir = "/tmp/build-test"
	return s
}

func (s *Setup) expectNoKustomize() *Setup {
	kustomizePath := filepath.Join(sourcePath, kustomizeFile)
	s.execProvider.EXPECT().FileExists(kustomizePath).Return(false).Times(1)
	return s
}

func (s *Setup) expectKubectlFound() *Setup {
	s.execProvider.EXPECT().FindOnPath(kubectl.KubectlCmd).Return(s.path, nil).Times(1)
	return s
}

func (s *Setup) expectKustomizeFound() *Setup {
	s.execProvider.EXPECT().FindOnPath(kubectl.KustomizeCmd).Return(s.path, nil).Times(1)
	return s
}

func (s *Setup) expectKubectlNotFound() *Setup {
	s.execProvider.EXPECT().FindOnPath(kubectl.KubectlCmd).Return(s.path, errExecFileNotFound).Times(1)
	return s
}

func (s *Setup) withKustomizeFactoryLogr(logger logr.Logger) *Setup {
	factory, err := kubectl.NewCommandFactory(logger, s.execProvider, kubectl.KustomizeCmd)
	s.t.Logf("kustomize (%T) %#v", factory, factory)
	if err != nil {
		s.t.Errorf("Error returned from the execLookPath function : %w", err)
	}
	s.factory = factory
	return s
}

func (s *Setup) withKustomizeFactory() *Setup {
	return s.withKustomizeFactoryLogr(s.testLogr)
}

func (s *Setup) withFactoryLogr(logger logr.Logger) *Setup {
	factory, err := kubectl.NewCommandFactory(logger, s.execProvider, kubectl.KubectlCmd)
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

func (s *Setup) expectBuildRun(extraArgs []string) *Setup {
	s.expectKustomizeFound()
	s.execProvider.EXPECT().ExecCmd(s.path, append(s.runArgs, extraArgs...)).Return([]byte(s.output), nil).Times(1)
	return s
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
	s.mockLogr.EXPECT().V(gomock.Any()).Return(s.mockLogr).Times(7)
	s.mockLogr.EXPECT().Info("Entering function", "function", "kubectl.(*CommandFactory).isKustomization", "source", "kubectl.go", "line", gomock.Any()).Times(1)
	s.mockLogr.EXPECT().Info("Exiting function", "function", "kubectl.(*CommandFactory).isKustomization", "source", "kubectl.go", "line", gomock.Any()).Times(1)
	s.mockLogr.EXPECT().Info("executing kubectl", "function", "kubectl.(*abstractCommand).Run", "source", "kubectl.go", "line", gomock.Any(), "command", s.runStr).Times(1)
	s.mockLogr.EXPECT().Info("Entering function", "function", "kubectl.(*abstractCommand).Run", "source", "kubectl.go", "line", gomock.Any()).Times(1)
	s.mockLogr.EXPECT().Info("Exiting function", "function", "kubectl.(*abstractCommand).Run", "source", "kubectl.go", "line", gomock.Any()).Times(1)
	s.mockLogr.EXPECT().Info("Entering function", "function", "kubectl.(*CommandFactory).Apply", "source", "kubectl.go", "line", gomock.Any()).Times(1)
	s.mockLogr.EXPECT().Info("Exiting function", "function", "kubectl.(*CommandFactory).Apply", "source", "kubectl.go", "line", gomock.Any()).Times(1)
	return s
}

func (s *Setup) expectDryRunLogsKubectlCommand() *Setup {
	s.expectDryRun()
	// The ApplyCommand should use the factory's mockLogger to log a log-leveled message indicating that kubectl was executed with the expected command
	s.mockLogr.EXPECT().V(gomock.Any()).Return(s.mockLogr).Times(7)
	s.mockLogr.EXPECT().Info("Entering function", "function", "kubectl.(*CommandFactory).isKustomization", "source", "kubectl.go", "line", gomock.Any()).Times(1)
	s.mockLogr.EXPECT().Info("Exiting function", "function", "kubectl.(*CommandFactory).isKustomization", "source", "kubectl.go", "line", gomock.Any()).Times(1)
	s.mockLogr.EXPECT().Info("executing kubectl", "function", "kubectl.(*abstractCommand).Run", "source", "kubectl.go", "line", gomock.Any(), "command", s.dryRunStr).Times(1)
	s.mockLogr.EXPECT().Info("Entering function", "function", "kubectl.(*abstractCommand).Run", "source", "kubectl.go", "line", gomock.Any()).Times(1)
	s.mockLogr.EXPECT().Info("Exiting function", "function", "kubectl.(*abstractCommand).Run", "source", "kubectl.go", "line", gomock.Any()).Times(1)
	s.mockLogr.EXPECT().Info("Entering function", "function", "kubectl.(*CommandFactory).Apply", "source", "kubectl.go", "line", gomock.Any()).Times(1)
	s.mockLogr.EXPECT().Info("Exiting function", "function", "kubectl.(*CommandFactory).Apply", "source", "kubectl.go", "line", gomock.Any()).Times(1)
	return s
}

func (s *Setup) expectRunUsesCommandLogger() *Setup {
	s.expectRun()
	// The ApplyCommand should use the command's mockLogger to log a log-leveled message indicating that kubectl was executed with the expected command
	s.cmdLogr.EXPECT().V(gomock.Any()).Return(s.cmdLogr).Times(3)
	s.cmdLogr.EXPECT().Info("executing kubectl", "function", "kubectl.(*abstractCommand).Run", "source", "kubectl.go", "line", gomock.Any(), "command", s.runStr).Times(1)
	s.cmdLogr.EXPECT().Info("Entering function", "function", "kubectl.(*abstractCommand).Run", "source", "kubectl.go", "line", gomock.Any()).Times(1)
	s.cmdLogr.EXPECT().Info("Exiting function", "function", "kubectl.(*abstractCommand).Run", "source", "kubectl.go", "line", gomock.Any()).Times(1)
	// The Factory's mockLogger should never be used
	s.mockLogr.EXPECT().V(gomock.Any()).Return(s.mockLogr).Times(4)
	s.mockLogr.EXPECT().Info(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(4)
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

func (s *Setup) Build() *kubectl.BuildCommand {
	command := s.factory.Build(s.sourceDir)
	s.validateCommand(command, &kubectl.BuildCommand{}, "Build")
	typedCommand, ok := command.(*kubectl.BuildCommand)
	if !ok {
		s.t.Logf("error casting to *kubectl.BuildCommand! %#v", command)
	}
	return typedCommand
}

func (s *Setup) Apply() *kubectl.ApplyCommand {
	command := s.factory.Apply(s.sourceDir)
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
