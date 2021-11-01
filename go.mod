module github.com/fidelity/kraan

go 1.14

require (
	github.com/fluxcd/helm-controller/api v0.11.2
	github.com/fluxcd/pkg/apis/meta v0.10.1
	github.com/fluxcd/pkg/untar v0.1.0
	github.com/fluxcd/source-controller/api v0.15.4
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v0.4.0
	github.com/golang/mock v1.5.0
	github.com/google/go-cmp v0.5.6
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.14.0
	github.com/paulcarlton-ww/goutils/pkg/kubectl v0.0.4
	github.com/paulcarlton-ww/goutils/pkg/testutils v0.1.42
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.0
	go.uber.org/zap v1.18.1
	golang.org/x/mod v0.4.2
	helm.sh/helm/v3 v3.6.3
	k8s.io/api v0.22.3
	k8s.io/apiextensions-apiserver v0.22.3
	k8s.io/apimachinery v0.22.3
	k8s.io/cli-runtime v0.21.3
	k8s.io/client-go v0.22.3
	k8s.io/kubectl v0.21.3
	sigs.k8s.io/controller-runtime v0.9.5
	sigs.k8s.io/kind v0.11.1
)

replace (
	github.com/docker/distribution => github.com/docker/distribution v0.0.0-20191216044856-a8371794149d
	github.com/docker/docker => github.com/moby/moby v17.12.0-ce-rc1.0.20200618181300-9dc6525e6118+incompatible
)
