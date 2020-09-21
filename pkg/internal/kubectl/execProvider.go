//Package kubectl executes various kubectl sub-commands in a forked shell
package kubectl

import (
	"os/exec"
)

// ExecProvider interface defines functions Kubectl uses to verify and execute a local command.
type ExecProvider interface {
	FindOnPath(file string) (string, error)
	ExecCmd(name string, arg ...string) ([]byte, error)
}

// OsExecProvider implements the ExecProvider interface using the os/exec go module.
type realExecProvider struct{}

// NewExecProvider returns an instance of OsExecProvider to implement the ExecProvider interface.
func newExecProvider() ExecProvider {
	return realExecProvider{}
}

func (p realExecProvider) FindOnPath(file string) (string, error) {
	return exec.LookPath(file)
}

func (p realExecProvider) ExecCmd(name string, arg ...string) ([]byte, error) {
	return exec.Command(name, arg...).Output()
}
