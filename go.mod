module github.com/fidelity/kraan

go 1.14

require (
	github.com/fluxcd/pkg/untar v0.0.5
	github.com/fluxcd/source-controller/api v0.0.13
	github.com/go-logr/logr v0.2.1
	github.com/golang/mock v1.4.4
	github.com/google/go-cmp v0.5.2
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	github.com/paulcarlton-ww/go-utils v0.0.0-20200729094929-4657992b390c // indirect
	github.com/pkg/errors v0.9.1
	go.uber.org/zap v1.16.0
	golang.org/x/mod v0.3.0
	golang.org/x/tools v0.0.0-20200930213115-e57f6d466a48 // indirect
	google.golang.org/protobuf v1.24.0 // indirect
	k8s.io/api v0.19.2
	k8s.io/apiextensions-apiserver v0.18.6
	k8s.io/apimachinery v0.19.2
	k8s.io/client-go v0.19.2
	sigs.k8s.io/controller-runtime v0.6.3
)
