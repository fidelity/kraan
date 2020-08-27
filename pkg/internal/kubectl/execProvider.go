/*Package kubectl xxx
 */
//  Old - mockgen -destination=pkg/internal/kubectl/mockExecProvider.go -package=kubectl -source=pkg/internal/kubectl/execProvider.go
//  github.com/fidelity/kraan/pkg/internal/kubectl ExecProvider
//go:generate mockgen -destination=mockExecProvider.go -package=kubectl -source=execProvider.go . ExecProvider
//go:generate mockgen -destination=../mocks/logr/mockLogger.go -package=mocks github.com/go-logr/logr Logger
package kubectl

import (
	"os/exec"
)

// ExecProvider interface defines functions Kubectl uses to verify and execute a local command.
type ExecProvider interface {
	findOnPath(file string) (string, error)
	execCmd(name string, arg ...string) ([]byte, error)
}

// OsExecProvider implements the ExecProvider interface using the os/exec go module.
type OsExecProvider struct{}

// NewExecProvider returns an instance of OsExecProvider to implement the ExecProvider interface.
func NewExecProvider() ExecProvider {
	return OsExecProvider{}
}

func (p OsExecProvider) findOnPath(file string) (string, error) {
	return exec.LookPath(file)
}

func (p OsExecProvider) execCmd(name string, arg ...string) ([]byte, error) {
	return exec.Command(name, arg...).Output()
}
