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

package kubectl

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/go-logr/logr"
)

// Kubectl provides an api for creating and running kubectl sub-commands.
type Kubectl struct {
	logger  logr.Logger
	cmdPath string
}

// NewKubectl returns a Kubectl object for creating and running kubectl sub-commands.
func NewKubectl(l logr.Logger) (kctl *Kubectl, err error) {
	kctl = &Kubectl{
		logger: l,
	}
	kctl.cmdPath, err = exec.LookPath("kubectl")
	if err != nil {
		err = fmt.Errorf("unable to find kubectl binary on system PATH: %w", err)
	}
	return kctl, err
}

// kubcetlCommand is an parent type holding common logic and fields for the sub-commands.
type kubectlCommand struct {
	logger     logr.Logger
	kctl       *Kubectl
	subCmd     string
	jsonOutput bool
	args       []string
	cmd        string
	output     []byte
}

func (c *kubectlCommand) logInfo(msg string, keysAndValues ...interface{}) {
	c.logger.V(1).Info(msg, append(keysAndValues, "command", c.asString())...)
}

func (c *kubectlCommand) logError(sourceErr error, keysAndValues ...interface{}) (err error) {
	msg := "error executing kubectl command"
	c.logger.Error(err, msg, append(keysAndValues, "command", c.asString())...)
	return fmt.Errorf("%s '%s' : %w", msg, c.asString(), sourceErr)
}

func (c *kubectlCommand) kubectlPath() (cmdPath string) {
	return c.kctl.cmdPath
}

func (c *kubectlCommand) kubectlArgs() (kargz []string) {
	return append([]string{c.subCmd}, c.args...)
}

func (c *kubectlCommand) asString() (cmdString string) {
	if c.cmd == "" {
		c.cmd = strings.Join(append([]string{c.kubectlPath()}, c.kubectlArgs()...), " ")
	}
	return c.cmd
}

// Run executes the Kubectl command with all its arguments and returns the output.
func (c *kubectlCommand) Run() (output []byte, err error) {
	if c.jsonOutput {
		c.args = append(c.args, "-o", "json")
	}
	c.logInfo("executing kubectl")
	c.output, err = exec.Command(c.kubectlPath(), c.kubectlArgs()...).CombinedOutput()
	if err != nil {
		err = c.logError(err)
	}
	return c.output, err
}

// DryRun executes the Kubectl command as a dry run and returns the output without making any changes to the cluster.
func (c *kubectlCommand) DryRun() (output []byte, err error) {
	c.args = append(c.args, "--dry-run")
	return c.Run()
}

// WithLogger sets the Logger the command should use to log actions if passed a Logger that is not nil.
func (c *kubectlCommand) WithLogger(logger logr.Logger) (self *kubectlCommand) {
	if logger != nil {
		c.logger = logger
	}
	return c
}

// ApplyCommand is a Kubectl sub-command that recursively applies all the YAML files it finds in a directory.
type ApplyCommand struct {
	kubectlCommand
}

// Apply instantiates an ApplyCommand instance using the provided directory path.
func (k *Kubectl) Apply(path string) (c *ApplyCommand) {
	c = &ApplyCommand{
		kubectlCommand: kubectlCommand{
			logger:     k.logger,
			kctl:       k,
			subCmd:     "apply",
			jsonOutput: true,
			args:       []string{"apply", "-R", "-f", path},
		},
	}
	return c
}
