/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

//Package kubectl executes various kubectl sub-commands in a forked shell
//go:generate mockgen -destination=../mocks/kubectl/mockKubectl.go -package=mocks . Kubectl,Command
package kubectl

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
)

const (
	kustomizeYaml = "kustomization.yaml"
)

var (
	applyArgs           = []string{"-R", "-f"}
	kustomizeApplyArgs  = []string{"-k"}
	kubectlCmd          = "kubectl"
	newExecProviderFunc = newExecProvider
)

// Kubectl is a Factory interface that returns concrete Command implementations from named constructors.
type Kubectl interface {
	Apply(path string) (c Command)
	Delete(args ...string) (c Command)
	Get(args ...string) (c Command)
}

// NewKubectl returns a Kubectl object for creating and running kubectl sub-commands.
func NewKubectl(logger logr.Logger) (kubectl Kubectl, err error) {
	execProvider := newExecProviderFunc()
	return newCommandFactory(logger, execProvider)
}

// CommandFactory is a concrete Factory implementation of the Kubectl interface's API.
type CommandFactory struct {
	logger       logr.Logger
	path         string
	execProvider ExecProvider
}

func newCommandFactory(logger logr.Logger, execProvider ExecProvider) (factory *CommandFactory, err error) {
	factory = &CommandFactory{
		logger:       logger,
		execProvider: execProvider,
	}
	factory.path, err = factory.getExecProvider().FindOnPath(kubectlCmd)
	if err != nil {
		err = fmt.Errorf("unable to find %s binary on system PATH: %w", kubectlCmd, err)
	}
	return factory, err
}

func (f CommandFactory) getLogger() logr.Logger {
	return f.logger
}

func (f CommandFactory) getPath() string {
	return f.path
}

func (f CommandFactory) getExecProvider() ExecProvider {
	return f.execProvider
}

// Command defines an interface for commands created by the Kubectl factory type.
type Command interface {
	Run() (output []byte, err error)
	DryRun() (output []byte, err error)
	WithLogger(logger logr.Logger) (self Command)
	getPath() string
	getSubCmd() string
	getArgs() []string
	asString() string
	isJSONOutput() bool
}

// abstractCommand is a parent type with common logic and fields used by concrete Command types.
type abstractCommand struct {
	logger     logr.Logger
	factory    *CommandFactory
	subCmd     string
	jsonOutput bool
	args       []string
	cmd        string
	output     []byte
}

func (c *abstractCommand) logInfo(msg string, keysAndValues ...interface{}) {
	c.logger.V(1).Info(msg, append(keysAndValues, "command", c.asString())...)
}

func (c *abstractCommand) logError(sourceErr error, keysAndValues ...interface{}) (err error) {
	msg := "error executing kubectl command"
	c.logger.Error(err, msg, append(keysAndValues, "command", c.asString())...)
	return fmt.Errorf("%s '%s' : %w", msg, c.asString(), sourceErr)
}

func (c *abstractCommand) getPath() (path string) {
	return c.factory.path
}

func (c *abstractCommand) getSubCmd() (subCmd string) {
	return c.subCmd
}

func (c *abstractCommand) getArgs() (kargz []string) {
	return append([]string{c.subCmd}, c.args...)
}

func (c *abstractCommand) isJSONOutput() bool {
	return c.jsonOutput
}

func (c *abstractCommand) asString() (cmdString string) {
	if c.cmd == "" {
		c.cmd = strings.Join(append([]string{c.getPath()}, c.getArgs()...), " ")
	}
	return c.cmd
}

// Run executes the Kubectl command with all its arguments and returns the output.
func (c *abstractCommand) Run() (output []byte, err error) {
	if c.jsonOutput {
		c.args = append(c.args, "-o", "json")
	}
	c.logInfo("executing kubectl")
	c.output, err = c.factory.getExecProvider().ExecCmd(c.getPath(), c.getArgs()...)
	if err != nil {
		err = c.logError(err)
	}
	return c.output, err
}

// DryRun executes the Kubectl command as a dry run and returns the output without making any changes to the cluster.
func (c *abstractCommand) DryRun() (output []byte, err error) {
	c.args = append(c.args, "--server-dry-run")
	return c.Run()
}

// WithLogger sets the Logger the command should use to log actions if passed a Logger that is not nil.
func (c *abstractCommand) WithLogger(logger logr.Logger) (self Command) {
	if logger != nil {
		c.logger = logger
	}
	return c
}

// ApplyCommand is a Kubectl sub-command that recursively applies all the YAML files it finds in a directory.
type ApplyCommand struct {
	abstractCommand
}

// Apply instantiates an ApplyCommand instance using the provided directory path.
func (f *CommandFactory) Apply(path string) (c Command) {
	c = &ApplyCommand{
		abstractCommand: abstractCommand{
			logger:     f.logger,
			factory:    f,
			subCmd:     "apply",
			jsonOutput: true,
			args:       f.applyArgs(path),
		},
	}
	return c
}

func (f *CommandFactory) applyArgs(dir string) []string {
	if f.getExecProvider().FileExists(filepath.Join(dir, kustomizeYaml)) {
		return append(kustomizeApplyArgs, dir)
	}
	return append(applyArgs, dir)
}

// DeleteCommand implements the Command interface to delete resources from the KubeAPI service.
type DeleteCommand struct {
	abstractCommand
}

// Delete instantiates a DeleteCommand instance for the described Kubernetes resource.
func (f *CommandFactory) Delete(args ...string) (c Command) {
	c = &DeleteCommand{
		abstractCommand: abstractCommand{
			logger:     f.logger,
			factory:    f,
			subCmd:     "delete",
			jsonOutput: false,
			args:       args,
		},
	}
	return c
}

// GetCommand implements the Command interface to delete resources from the KubeAPI service
type GetCommand struct {
	abstractCommand
}

// Get instantiates a GetCommand instance for the described Kubernetes resource
func (f *CommandFactory) Get(args ...string) (c Command) {
	c = &GetCommand{
		abstractCommand: abstractCommand{
			logger:     f.logger,
			factory:    f,
			subCmd:     "get",
			jsonOutput: true,
			args:       args,
		},
	}
	return c
}
