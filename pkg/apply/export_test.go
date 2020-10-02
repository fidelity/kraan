package apply

import (
	"github.com/fidelity/kraan/pkg/internal/kubectl"
	"github.com/go-logr/logr"
)

func SetNewKubectlFunc(kubectlFunc func(logger logr.Logger) (kubectl.Kubectl, error)) {
	newKubectlFunc = kubectlFunc
}
