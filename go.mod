module github.com/fidelity/kraan

go 1.14

require (
	github.com/fluxcd/helm-controller/api v0.0.10
	github.com/fluxcd/pkg/untar v0.0.5
	github.com/fluxcd/source-controller/api v0.0.13
	github.com/go-logr/logr v0.2.1
	github.com/golang/mock v1.4.4
	github.com/google/go-cmp v0.5.2
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	github.com/paulcarlton-ww/go-utils v0.0.0-20200729094929-4657992b390c
	go.uber.org/zap v1.16.0
	golang.org/x/mod v0.1.1-0.20191105210325-c90efee705ee
	k8s.io/api v0.19.2
	k8s.io/apimachinery v0.19.2
	k8s.io/client-go v0.19.2
	sigs.k8s.io/controller-runtime v0.6.3
)

replace (
	k8s.io/api => k8s.io/api v0.19.2
	k8s.io/apimachinery => k8s.io/apimachinery v0.19.2
	k8s.io/client-go => k8s.io/client-go v0.19.2
	github.com/go-logr/zapr => github.com/go-logr/zapr v0.2.0
)
