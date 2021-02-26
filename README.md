# kraan - Building platforms on top of K8s

*This project is currently in the early stages of development and expected to release
beta versions by end of September*.

## What is kraan?

kraan helps you deploy and manage *'layers'* on top of kubernetes. By applying *layers*
on top of K8s clusters, you can build focused platforms on top of
K8s e.g ML platforms, Data platform etc. Each *layer* is a collection of addons and
can have dependencies established between the layers. i.e a "mgmt-layer"
can depend on a "common-layer". Kraan will always ensure that the addons in the "common-layer" are deployed successfully before deploying
the "mgmt-layer" addons. A layer is represented as a kubernetes custom resource and
kraan is an [operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) that
is deployed into the cluster and works constantly to reconcile the state of the
layer custom resource.

kraan is powered by [flux2](https://toolkit.fluxcd.io/) and builds on
top of projects like [source-controller](https://github.com/fluxcd/source-controller)
and [helm-controller](https://github.com/fluxcd/helm-controller).

## Use cases

Kraan can be used wherever you have requirements to manage add-ons on top of k8s
clusters and especially when you want to package the addons into dependant categories.
If you have a mutating webhook injecting a side-car and set of security
plugins which should always be deployed first before other addons,then *layers*
concept in Kraan will help there. However, Kraan is even more powerful when it comes
to building custom platforms on top of k8s like the one shown below.

kraan promotes the idea of building model based platforms on top of k8s i.e you can
build a "general purpose" k8s platform which might have a "common" and "security"
layers which packages all the common tooling, applications inside that cluster might
need as well as organization specific bits (e.g org specific security addons etc).
You can also say that the "common-layer" *depends-on* "security-layer" to be deployed first.
This "general purpose" k8s platform can then be *extended* by applying another "ml-layer"
which can then be exposed as an ML platform to the development teams. The end result here
is developers working on top of secure and custom-built platforms which adheres
to organization specific policies etc. And rolling out updates to this ML
platform is as simple as "kubectl apply -f <LAYERS_DIR>" where new versions of the layers are
deployed into the cluster and kraan operator will constantly work towards
getting that platform to match the latest desired state!

The below diagram shows how you can use kraan to build a focused multi-cloud
platform where "common" and "security" layers are shared across clouds whereas
other layers become cloud specific.

![custom-platform](docs/diagrams/custom-platform.png)

## Design

Kraan is a kubernetes controller that is built on top of [k8s custom resources](
https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/).
It works in tandem with [source-controller](https://github.com/fluxcd/source-controller)
and [helm-controller](https://github.com/fluxcd/helm-controller) and hence they are always
deployed together. The detailed design documentation can be found [here](docs/design/README.md)

## Usage

See [User Guide](docs/user-guide.md) for usage instructions.

See [Developer Guide](docs/dev-guide.md) for development.

## Contributions

Contributions are very welcome. Please read the [contributing guide](CONTRIBUTING.md) or see the docs.
