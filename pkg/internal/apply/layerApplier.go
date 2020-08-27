//Package apply xxx
// TEMP DISABLED - go:generate mockgen -destination=mockLayerApplier.go -package=apply -source=layerApplier.go . LayerApplier
// re-enable the go:generate annotation when we're ready to write tests for the controller
package apply

/*
To generate mock code for the LayerApplier run 'go generate ./...' from the project root directory.
*/
import (
	"context"
	"fmt"
	"os"

	"github.com/fidelity/kraan/pkg/internal/kubectl"
	"github.com/fidelity/kraan/pkg/internal/layers"

	helmopv1 "github.com/fluxcd/helm-operator/pkg/apis/helm.fluxcd.io/v1"

	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	newKubectlFunc func(logger logr.Logger) (kubectl.Kubectl, error) = kubectl.NewKubectl
)

// LayerApplier defines methods for managing the Addons within an AddonLayer in a cluster.
type LayerApplier interface {
	Apply(layer layers.Layer) (err error)
	Prune(layer layers.Layer) (err error)
}

// KubectlLayerApplier applies an AddonsLayer to a Kubernetes cluster using the kubectl command.
type KubectlLayerApplier struct {
	client  client.Client
	kubectl kubectl.Kubectl
	scheme  *runtime.Scheme
	logger  logr.Logger
}

// NewApplier returns a LayerApplier instance.
func NewApplier(client client.Client, logger logr.Logger, scheme *runtime.Scheme) (applier LayerApplier, err error) {
	kubectl, err := newKubectlFunc(logger)
	if err != nil {
		return nil, err
	}
	applier = KubectlLayerApplier{
		client:  client,
		kubectl: kubectl,
		scheme:  scheme,
		logger:  logger,
	}
	return applier, nil
}

func (a KubectlLayerApplier) getLog(layer layers.Layer) (logger logr.Logger) {
	logger = layer.GetLogger()
	if logger == nil {
		logger = a.logger
	}
	return logger
}

func (a KubectlLayerApplier) log(level int, msg string, layer layers.Layer, keysAndValues ...interface{}) {
	a.getLog(layer).V(level).Info(msg, append(keysAndValues, "sourcePath", layer.GetSourcePath(), "layer", layer)...)
}

func (a KubectlLayerApplier) logError(err error, msg string, layer layers.Layer, keysAndValues ...interface{}) {
	a.getLog(layer).Error(err, msg, append(keysAndValues, "sourcePath", layer.GetSourcePath(), "layer", layer)...)
}

func (a KubectlLayerApplier) logInfo(msg string, layer layers.Layer, keysAndValues ...interface{}) {
	a.log(1, msg, layer, keysAndValues...)
}

func (a KubectlLayerApplier) logDebug(msg string, layer layers.Layer, keysAndValues ...interface{}) {
	a.log(5, msg, layer, keysAndValues...)
}

func (a KubectlLayerApplier) logTrace(msg string, layer layers.Layer, keysAndValues ...interface{}) {
	a.log(7, msg, layer, keysAndValues...)
}

func (a KubectlLayerApplier) logErrors(errz []error, layer layers.Layer) {
	for _, err := range errz {
		a.logError(err, "error while applying layer", layer)
	}
}

func (a KubectlLayerApplier) logAddons(hrs []*helmopv1.HelmRelease, layer layers.Layer) error {
	for i, hr := range hrs {
		a.logInfo("Deployed HelmRelease for AddonsLayer", layer, "index", i, "helmRelease", hr)
		key, err := client.ObjectKeyFromObject(hr)
		if err != nil {
			return fmt.Errorf("unable to get an ObjectKey from HelmRelease '%s'", getLabel(hr))
		}
		foundHr := &helmopv1.HelmRelease{}
		err = a.client.Get(context.Background(), key, foundHr)
		if err != nil {
			return fmt.Errorf("failed to Get HelmRelease '%s'", getLabel(hr))
		}
		fmt.Printf("Found HelmRelease '%s'\n", getLabel(hr))
	}
	return nil
}

func getLabel(hr *helmopv1.HelmRelease) string {
	return fmt.Sprintf("%s/%s", hr.GetNamespace(), hr.GetName())
}

func (a KubectlLayerApplier) decodeAddons(layer layers.Layer,
	json []byte) (hrs []*helmopv1.HelmRelease, errz []error, err error) {
	// TODO - should probably trace log the json before we try to decode it.
	a.logTrace("decoding JSON output from kubectl", layer, "output", json)

	// dez := a.scheme.Codecs.UniversalDeserializer()
	dez := serializer.NewCodecFactory(a.scheme).UniversalDeserializer()

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
		hr, ok := obj.(*helmopv1.HelmRelease)
		if !ok {
			return nil, nil, fmt.Errorf("unable to cast HelmRelease typed object to a HelmRelease type")
		}
		a.logDebug("decoded single HelmRelease from kubectl output", layer, "groupVersionKind", gvk, "helmRelease", hr)
		return []*helmopv1.HelmRelease{hr}, nil, nil
	default:
		msg := "decoded kubectl output was not a HelmRelease or List"
		err = fmt.Errorf(msg)
		a.logError(err, msg, layer, "output", json, "groupVersionKind", gvk, "object", obj)
		return nil, nil, err
	}
}

func (a KubectlLayerApplier) addOwnerRefs(layer layers.Layer, hrs []*helmopv1.HelmRelease) error {
	for i, hr := range hrs {
		a.logDebug("Adding owner ref to HelmRelease for AddonsLayer", layer, "index", i, "helmRelease", hr)
		err := controllerutil.SetControllerReference(layer.GetAddonsLayer(), hr, a.scheme)
		if err != nil {
			// could not apply owner ref for object
			return fmt.Errorf("unable to apply owner reference for AddonsLayer '%s' to HelmRelease '%s': %w", layer.GetName(), getLabel(hr), err)
		}
	}
	return nil
}

func (a KubectlLayerApplier) getHelmRelease(hr *helmopv1.HelmRelease) (*helmopv1.HelmRelease, error) {
	key, err := client.ObjectKeyFromObject(hr)
	if err != nil {
		return nil, fmt.Errorf("unable to get an ObjectKey from HelmRelease '%s'", getLabel(hr))
	}
	foundHr := &helmopv1.HelmRelease{}
	err = a.client.Get(context.Background(), key, foundHr)
	if err != nil {
		return nil, fmt.Errorf("failed to Get HelmRelease '%s'", getLabel(hr))
	}
	return foundHr, nil
}

func (a KubectlLayerApplier) applyHelmReleases(layer layers.Layer, hrs []*helmopv1.HelmRelease) error {
	for i, hr := range hrs {
		a.logDebug("Applying HelmRelease for AddonsLayer", layer, "index", i, "helmRelease", hr)
		foundHr, err := a.getHelmRelease(hr)
		if err != nil {
			// TODO - not sure if logic goes here if the HelmRelease does not exist on the cluster

		} else if foundHr == nil {
			// HelmRelease does not exist, create resource
			err = a.client.Create(context.Background(), hr, &client.CreateOptions{})
			if err != nil {
				return fmt.Errorf("unable to Create HelmRelease '%s' on the target cluster", getLabel(hr))
			}
		} else {
			// HelmRelease exists, update resource
			err = a.client.Update(context.Background(), hr, &client.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("unable to Update HelmRelease '%s' on the target cluster", getLabel(hr))
			}
		}
	}
	return nil
}

func (a KubectlLayerApplier) decodeList(layer layers.Layer,
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

func (a KubectlLayerApplier) checkSourcePath(layer layers.Layer) (sourceDir string, err error) {
	sourceDir = layer.GetSourcePath()
	info, err := os.Stat(sourceDir)
	if os.IsNotExist(err) {
		a.logDebug("source directory not found", layer)
		return sourceDir, fmt.Errorf("source directory (%s) not found for AddonsLayer %s/%s",
			sourceDir, "", layer.GetName())
	}
	if os.IsPermission(err) {
		a.logDebug("source directory read permission denied", layer)
		return sourceDir, fmt.Errorf("read permission denied to source directory (%s) for AddonsLayer %s/%s",
			sourceDir, "", layer.GetName())
	}
	if err != nil {
		a.logError(err, "error while checking source directory", layer)
		return sourceDir, fmt.Errorf("error while checking source directory (%s) for AddonsLayer %s/%s",
			sourceDir, "", layer.GetName())
	}
	if !info.IsDir() {
		// I'm not sure if this is an error, but I thought I should detect and log it
		a.logInfo("source path is not a directory", layer)
	}
	return sourceDir, nil
}

// Apply an AddonLayer to the cluster.
func (a KubectlLayerApplier) Apply(layer layers.Layer) (err error) {
	a.logInfo("Applying AddonsLayer", layer)

	sourceDir, err := a.checkSourcePath(layer)
	if err != nil {
		return err
	}

	//output, err := a.kubectl.Apply(sourceDir).WithLogger(layer.GetLogger()).Run()
	output, err := a.kubectl.Apply(sourceDir).WithLogger(layer.GetLogger()).DryRun()
	if err != nil {
		return fmt.Errorf("error from kubectl while parsing source directory (%s) for AddonsLayer %s/%s",
			sourceDir, "", layer.GetName())
	}

	hrs, errz, err := a.decodeAddons(layer, output)
	if err != nil {
		if errz != nil && len(errz) > 0 {
			a.logErrors(errz, layer)
		}
		return err
	}

	// TODO: Add an ownerRef to each deployed HelmRelease the points back to this AddonsLayer (needed for Prune)
	err = a.addOwnerRefs(layer, hrs)
	if err != nil {
		return err
	}

	err = a.applyHelmReleases(layer, hrs)
	if err != nil {
		return err
	}

	err = a.logAddons(hrs, layer)
	if err != nil {
		return err
	}

	// TODO: Watch all HelmRelease resources applied for this AddonsLayer until all are success or fail or timeout

	return nil
}

// ApplyIsRequired returns true if any resources need to be applied for this AddonsLayer
func (a KubectlLayerApplier) ApplyIsRequired(layer layers.Layer) (applyIsRequired bool, err error) {
	return true, nil
}

// PruneIsRequired returns true if any resources need to be prunedfor this AddonsLayer
func (a KubectlLayerApplier) PruneIsRequired(layer layers.Layer) (applyIsRequired bool, err error) {
	return false, nil
}

// Prune the AddonsLayer by removing the Addons found in the cluster that have since been removed from the Layer.
func (a KubectlLayerApplier) Prune(layer layers.Layer) (err error) {
	// TODO: Stub method placeholder.
	// Get a list of HelmRelease resources described in the YAML files in the layer's sourceDirectory from the output of kubectl apply -R -f <sourceDir> --dry-run -o json
	// Get a list of HelmRelease resources in the cluster with ownerRefs to this AddonsLayer
	// Remove all HelmRelease resources found in the sourceDirectory from the list of HelmRelease resources in the cluster
	// Delete all HelmRelease resources in the remaining list from the cluster
	// Wait until Helm Operator finishes removing all deleted HelmRelease resources from the cluster (respect the timeout)
	// Return an error if any HelmRelease resources could not be deleted or the timeout has been exceeded
	return nil
}
