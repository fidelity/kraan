module github.com/fidelity/kraan

go 1.14

require (
	github.com/fluxcd/helm-controller/api v0.3.0
	github.com/fluxcd/pkg/apis/meta v0.4.0
	github.com/fluxcd/pkg/untar v0.0.5
	github.com/fluxcd/source-controller/api v0.3.0
	github.com/go-logr/logr v0.4.0
	github.com/go-logr/zapr v0.4.0 // indirect
	github.com/golang/mock v1.4.4
	github.com/google/go-cmp v0.5.2
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	github.com/paulcarlton-ww/goutils/pkg/testutils v0.1.42
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.7.1
	go.uber.org/zap v1.16.0
	golang.org/x/mod v0.3.0
	golang.org/x/tools v0.0.0-20201002184944-ecd9fd270d5d // indirect
	k8s.io/api v0.19.4
	k8s.io/apiextensions-apiserver v0.19.4
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.19.4
	sigs.k8s.io/controller-runtime v0.6.3
)
