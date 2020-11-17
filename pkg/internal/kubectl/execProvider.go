//Package kubectl executes various kubectl sub-commands in a forked shell
//go:generate mockgen -destination=../mocks/kubectl/mockExecProvider.go -package=mocks -source=execProvider.go . ExecProvider
package kubectl

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/fidelity/kraan/pkg/logging"
)

// ExecProvider interface defines functions Kubectl uses to verify and execute a local command.
type ExecProvider interface {
	FileExists(filePath string) bool
	FindOnPath(file string) (string, error)
	ExecCmd(name string, arg ...string) ([]byte, error)
}

// OsExecProvider implements the ExecProvider interface using the os/exec go module.
type realExecProvider struct{}

// NewExecProvider returns an instance of OsExecProvider to implement the ExecProvider interface.
func newExecProvider() ExecProvider {
	return realExecProvider{}
}

func (p realExecProvider) FileExists(filePath string) bool {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return false
	}
	return true
}

func (p realExecProvider) FindOnPath(file string) (string, error) {
	return exec.LookPath(file)
}

func (p realExecProvider) ExecCmd(name string, arg ...string) ([]byte, error) {
	data, err := exec.Command(name, arg...).Output()
	if err != nil {
		exitError, ok := err.(*exec.ExitError)
		if ok {
			return nil, fmt.Errorf("%s - kubectl failed, %s, error\n%s", logging.CallerStr(logging.Me), exitError.ProcessState.String(), string(exitError.Stderr))
		}
		return nil, err
	}
	return data, err
}
