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
//   OLD - mockgen -destination=pkg/internal/kubectl/mockKubectl.go -package=kubectl -source=pkg/internal/kubectl/kubectl.go
//   github.com/fidelity/kraan/pkg/internal/kubectl Kubectl,Command
//go:generate mockgen -destination=mockKubectl.go -package=kubectl -source=kubectl.go . Kubectl,Command
package kubectl

import (
	"fmt"
	"strings"

	"github.com/go-logr/logr"
)

var (
	kubectlCmd   = "kubectl"
	execProvider = NewExecProvider()
)

// Kubectl is a Factory interface that returns concrete Command implementations from named constructors.
type Kubectl interface {
	Apply(path string) (c Command)
	Delete(args ...string) (c Command)
	Get(args ...string) (c Command)
	getLogger() logr.Logger
	getPath() string
}

// CommandFactory is a concrete Factory implementation of the Kubectl interface's API.
type CommandFactory struct {
	logger logr.Logger
	path   string
}

// NewKubectl returns a Kubectl object for creating and running kubectl sub-commands.
func NewKubectl(logger logr.Logger) (factory Kubectl, err error) {
	kFactory := CommandFactory{
		logger: logger,
	}
	kFactory.path, err = execProvider.findOnPath(kubectlCmd)
	if err != nil {
		err = fmt.Errorf("unable to find %s binary on system PATH: %w", kubectlCmd, err)
	}
	return &kFactory, err
}

func (f CommandFactory) getLogger() logr.Logger {
	return f.logger
}

func (f CommandFactory) getPath() string {
	return f.path
}

// Command defines an interface for commands created by the Kubectl factory type.
type Command interface {
	Run() (output []byte, err error)
	DryRun() (output []byte, err error)
	WithLogger(logger logr.Logger) (self *abstractCommand)
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

func (c *abstractCommand) kubectlPath() (path string) {
	return c.factory.path
}

func (c *abstractCommand) kubectlSubCmd() (subCmd string) {
	return c.subCmd
}

func (c *abstractCommand) kubectlArgs() (kargz []string) {
	return append([]string{c.subCmd}, c.args...)
}

func (c *abstractCommand) asString() (cmdString string) {
	if c.cmd == "" {
		c.cmd = strings.Join(append([]string{c.kubectlPath()}, c.kubectlArgs()...), " ")
	}
	return c.cmd
}

// Run executes the Kubectl command with all its arguments and returns the output.
func (c *abstractCommand) Run() (output []byte, err error) {
	if c.jsonOutput {
		c.args = append(c.args, "-o", "json")
	}
	c.logInfo("executing kubectl")
	c.output, err = execProvider.execCmd(c.kubectlPath(), c.kubectlArgs()...)
	if err != nil {
		err = c.logError(err)
	}
	return c.output, err
}

// DryRun executes the Kubectl command as a dry run and returns the output without making any changes to the cluster.
func (c *abstractCommand) DryRun() (output []byte, err error) {
	c.args = append(c.args, "--dry-run")
	return c.Run()
}

// WithLogger sets the Logger the command should use to log actions if passed a Logger that is not nil.
func (c *abstractCommand) WithLogger(logger logr.Logger) (self *abstractCommand) {
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
			args:       []string{"-R", "-f", path},
		},
	}
	return c
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
	c = &DeleteCommand{
		abstractCommand: abstractCommand{
			logger:     f.logger,
			factory:    f,
			subCmd:     "get",
			jsonOutput: false,
			args:       args,
		},
	}
	return c
}
