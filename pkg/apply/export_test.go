package apply

import (
	"github.com/go-logr/logr"

	"github.com/fidelity/kraan/pkg/internal/kubectl"
)

func SetNewKubectlFunc(kubectlFunc func(logger logr.Logger) (kubectl.Kubectl, error)) {
	newKubectlFunc = kubectlFunc
}
