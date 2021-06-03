module github.com/fidelity/kraan

go 1.14

require (
	github.com/fluxcd/helm-controller/api v0.10.1
	github.com/fluxcd/pkg/apis/meta v0.9.0
	github.com/fluxcd/pkg/untar v0.0.5
	github.com/fluxcd/source-controller/api v0.12.2
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v0.4.0
	github.com/go-logr/zapr v0.4.0 // indirect
	github.com/golang/mock v1.5.0
	github.com/google/go-cmp v0.5.2
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/paulcarlton-ww/goutils/pkg/kubectl v0.0.4
	github.com/paulcarlton-ww/goutils/pkg/testutils v0.1.42
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.7.1
	go.uber.org/zap v1.16.0
	golang.org/x/mod v0.4.2
	golang.org/x/tools v0.0.0-20201002184944-ecd9fd270d5d // indirect
	helm.sh/helm/v3 v3.5.2
	k8s.io/api v0.20.2
	k8s.io/apiextensions-apiserver v0.20.2
	k8s.io/apimachinery v0.20.4
	k8s.io/cli-runtime v0.20.2
	k8s.io/client-go v0.20.2
	k8s.io/kubectl v0.20.1
	sigs.k8s.io/controller-runtime v0.8.3
	sigs.k8s.io/kind v0.10.0
)

replace (
	github.com/docker/distribution => github.com/docker/distribution v0.0.0-20191216044856-a8371794149d
	github.com/docker/docker => github.com/moby/moby v17.12.0-ce-rc1.0.20200618181300-9dc6525e6118+incompatible
)
