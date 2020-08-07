package actors

import (
	"fmt"
	"os"

	"github.com/fidelity/kraan/pkg/internal/kubectl"
	"github.com/fidelity/kraan/pkg/internal/layers"

	helmopv1 "github.com/fluxcd/helm-operator/pkg/apis/helm.fluxcd.io/v1"
	helmopscheme "github.com/fluxcd/helm-operator/pkg/client/clientset/versioned/scheme"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kscheme "k8s.io/client-go/kubernetes/scheme"
)

func init() {
	helmopscheme.AddToScheme(kscheme.Scheme) // nolint:errcheck // ok
}

// LayerApplier defines an Apply method to apply AddonLayers to a cluster.
type LayerApplier interface {
	Apply(layer *layers.Layer) (err error)
}

// KubectlLayerApplier applies an AddonsLayer to a Kubernetes cluster using the kubectl command.
type KubectlLayerApplier struct {
	kubectl *kubectl.Kubectl
	logger  logr.Logger
}

// NewApplier returns a LayerApplier instance.
func NewApplier(logger logr.Logger) (applier LayerApplier, err error) {
	kubectl, err := kubectl.NewKubectl(logger)
	if err != nil {
		return nil, err
	}
	applier = KubectlLayerApplier{
		kubectl: kubectl,
		logger:  logger,
	}
	return applier, nil
}

func (a KubectlLayerApplier) getLog(layer *layers.Layer) (logger logr.Logger) {
	logger = layer.GetLogger()
	if logger == nil {
		logger = a.logger
	}
	return logger
}

func (a KubectlLayerApplier) log(level int, msg string, layer *layers.Layer, keysAndValues ...interface{}) {
	a.getLog(layer).V(level).Info(msg, append(keysAndValues, "sourcePath", layer.GetSourcePath(), "layer", layer)...)
}

func (a KubectlLayerApplier) logError(err error, msg string, layer *layers.Layer, keysAndValues ...interface{}) {
	a.getLog(layer).Error(err, msg, append(keysAndValues, "sourcePath", layer.GetSourcePath(), "layer", layer)...)
}

func (a KubectlLayerApplier) logInfo(msg string, layer *layers.Layer, keysAndValues ...interface{}) {
	a.log(1, msg, layer, keysAndValues...)
}

func (a KubectlLayerApplier) logDebug(msg string, layer *layers.Layer, keysAndValues ...interface{}) {
	a.log(5, msg, layer, keysAndValues...)
}

func (a KubectlLayerApplier) logTrace(msg string, layer *layers.Layer, keysAndValues ...interface{}) {
	a.log(7, msg, layer, keysAndValues...)
}

// Apply an AddonLayer to the cluster.
func (a KubectlLayerApplier) Apply(layer *layers.Layer) (err error) {
	sourceDir := layer.GetSourcePath()
	a.logInfo("Applying AddonsLayer", layer)
	info, err := os.Stat(sourceDir)
	if os.IsNotExist(err) {
		a.logDebug("source directory not found", layer)
		return fmt.Errorf("source directory (%s) not found for AddonsLayer %s/%s",
			sourceDir, layer.GetNamespace(), layer.GetName())
	}
	if os.IsPermission(err) {
		a.logDebug("source directory read permission denied", layer)
		return fmt.Errorf("read permission denied to source directory (%s) for AddonsLayer %s/%s",
			sourceDir, layer.GetNamespace(), layer.GetName())
	}
	if err != nil {
		a.logError(err, "error while checking source directory", layer)
		return fmt.Errorf("error while checking source directory (%s) for AddonsLayer %s/%s",
			sourceDir, layer.GetNamespace(), layer.GetName())
	}
	if !info.IsDir() {
		// I'm not sure if this is an error, but I thought I should detect and log it
		a.logInfo("source path is not a directory", layer)
	}

	output, err := a.kubectl.Apply(sourceDir).WithLogger(layer.GetLogger()).Run()
	if err != nil {
		return fmt.Errorf("error from kubectl while applying source directory (%s) for AddonsLayer %s/%s",
			sourceDir, layer.GetNamespace(), layer.GetName())
	}

	hrs, errz, err := a.decodeAddons(layer, output)
	if err != nil {
		if errz != nil && len(errz) > 0 {
			a.logErrors(errz, layer)
		}
		return err
	}

	a.logAddons(hrs, layer)

	return nil
}

func (a KubectlLayerApplier) logErrors(errz []error, layer *layers.Layer) {
	for _, err := range errz {
		a.logError(err, "error while applying layer", layer)
	}
}

func (a KubectlLayerApplier) logAddons(hrs []*helmopv1.HelmRelease, layer *layers.Layer) {
	for i, hr := range hrs {
		a.logInfo("Deployed HelmRelease for AddonsLayer", layer, "index", i, "helmRelease", hr)
	}
}

func (a KubectlLayerApplier) decodeAddons(layer *layers.Layer,
	json []byte) (hrs []*helmopv1.HelmRelease, errz []error, err error) {
	// TODO - should probably trace log the json before we try to decode it.
	a.logTrace("decoding JSON output from kubectl", layer, "output", json)

	dez := kscheme.Codecs.UniversalDeserializer()

	obj, gvk, err := dez.Decode(json, nil, nil)
	if err != nil {
		a.logError(err, "unable to parse JSON output from kubectl", layer, "output", json)
		return nil, nil, err
	}

	a.logTrace("decoded JSON output", layer, "groupVersionKind", gvk, "object", obj)

	switch obj.(type) {
	case *corev1.List:
		a.logDebug("decoded raw object List from kubectl output", layer, "groupVersionKind", gvk, "list", obj)
		return a.decodeList(layer, obj.(*corev1.List), &dez)
	case *helmopv1.HelmRelease:
		hr := obj.(*helmopv1.HelmRelease) // nolint:errcheck
		a.logDebug("decoded single HelmRelease from kubectl output", layer, "groupVersionKind", gvk, "helmRelease", hr)
		return []*helmopv1.HelmRelease{hr}, nil, nil
	default:
		msg := "decoded kubectl output was not a HelmRelease or List"
		err = fmt.Errorf(msg)
		a.logError(err, msg, layer, "output", json, "groupVersionKind", gvk, "object", obj)
		return nil, nil, err
	}
}

func (a KubectlLayerApplier) decodeList(layer *layers.Layer,
	raws *corev1.List, dez *runtime.Decoder) (hrs []*helmopv1.HelmRelease, errz []error, err error) {
	dec := *dez

	a.logDebug("decoding list of raw JSON items", layer, "length", len(raws.Items))

	for i, raw := range raws.Items {
		obj, gvk, err := dec.Decode(raw.Raw, nil, nil)
		if err != nil {
			errz = append(errz, err)
		}
		switch obj.(type) {
		case *helmopv1.HelmRelease:
			hr := obj.(*helmopv1.HelmRelease) // nolint:errcheck
			a.logDebug("decoded HelmRelease from kubectl output list",
				layer, "index", i, "groupVersionKind", gvk, "helmRelease", hr)
			hrs = append(hrs, hr)
		default:
			a.logInfo("decoded Kubernetes object from kubectl output list",
				layer, "index", i, "groupVersionKind", gvk, "object", obj)
		}
	}
	if len(errz) > 0 {
		return hrs, errz, errz[0]
	}
	return hrs, nil, nil
}
