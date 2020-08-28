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
	"reflect"

	"github.com/fidelity/kraan/pkg/internal/kubectl"
	"github.com/fidelity/kraan/pkg/internal/layers"

	helmopv1 "github.com/fluxcd/helm-operator/pkg/apis/helm.fluxcd.io/v1"

	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	ownerLabel     string                                            = "kraan/owner"
	newKubectlFunc func(logger logr.Logger) (kubectl.Kubectl, error) = kubectl.NewKubectl
)

// LayerApplier defines methods for managing the Addons within an AddonLayer in a cluster.
type LayerApplier interface {
	Apply(layer layers.Layer) (err error)
	Prune(layer layers.Layer, pruneHrs []*helmopv1.HelmRelease) (err error)
	PruneIsRequired(layer layers.Layer) (pruneRequired bool, pruneHrs []*helmopv1.HelmRelease, err error)
	ApplyIsRequired(layer layers.Layer) (applyIsRequired bool, err error)
	ApplyWasSuccessful(layer layers.Layer) (applyIsRequired bool, err error)
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
		labels := hr.GetObjectMeta().GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		labels[ownerLabel] = layer.GetName()
		hr.ObjectMeta.Labels = labels
	}
	return nil
}

func (a KubectlLayerApplier) getHelmReleases(layer layers.Layer) (foundHrs []*helmopv1.HelmRelease, err error) {
	hrList := &helmopv1.HelmReleaseList{}
	err = a.client.List(context.Background(), hrList, client.MatchingFields{".owner": layer.GetName()})
	if err != nil {
		return nil, fmt.Errorf("unable to list HelmRelease resources owned by '%s': %w", layer.GetName(), err)
	}
	for _, hr := range hrList.Items {
		foundHrs = append(foundHrs, hr.DeepCopy())
	}
	return foundHrs, nil
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
		if err != nil || foundHr == nil {
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

func (a KubectlLayerApplier) checkHelmReleases(layer layers.Layer, hrs []*helmopv1.HelmRelease) error {
	for i, hr := range hrs {
		a.logInfo("Checking HelmRelease for AddonsLayer", layer, "index", i, "helmRelease", hr)
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

func (a KubectlLayerApplier) getSourceResources(layer layers.Layer) (hrs []*helmopv1.HelmRelease, err error) {
	sourceDir, err := a.checkSourcePath(layer)
	if err != nil {
		return nil, err
	}

	//output, err := a.kubectl.Apply(sourceDir).WithLogger(layer.GetLogger()).Run()
	output, err := a.kubectl.Apply(sourceDir).WithLogger(layer.GetLogger()).DryRun()
	if err != nil {
		return nil, fmt.Errorf("error from kubectl while parsing source directory (%s) for AddonsLayer %s/%s",
			sourceDir, "", layer.GetName())
	}

	hrs, errz, err := a.decodeAddons(layer, output)
	if err != nil {
		if errz != nil && len(errz) > 0 {
			a.logErrors(errz, layer)
		}
		return nil, err
	}

	// TODO: Add an ownerRef to each deployed HelmRelease the points back to this AddonsLayer (needed for Prune)
	err = a.addOwnerRefs(layer, hrs)
	if err != nil {
		return nil, err
	}

	return hrs, nil
}

// Apply an AddonLayer to the cluster.
func (a KubectlLayerApplier) Apply(layer layers.Layer) (err error) {
	a.logInfo("Applying AddonsLayer", layer)

	hrs, err := a.getSourceResources(layer)
	if err != nil {
		return err
	}

	err = a.applyHelmReleases(layer, hrs)
	if err != nil {
		return err
	}

	err = a.checkHelmReleases(layer, hrs)
	if err != nil {
		return err
	}

	return nil
}

// Prune the AddonsLayer by removing the Addons found in the cluster that have since been removed from the Layer.
func (a KubectlLayerApplier) Prune(layer layers.Layer, pruneHrs []*helmopv1.HelmRelease) (err error) {
	for _, hr := range pruneHrs {
		//err := a.client.Delete(context.Background(), hr)
		err := a.client.Delete(context.Background(), hr, client.PropagationPolicy(metav1.DeletePropagationBackground))
		if err != nil {
			return fmt.Errorf("unable to delete HelmRelease '%s' for AddonsLayer '%s'", getLabel(hr), layer.GetName())
		}
	}
	return nil
}

// PruneIsRequired returns true if any resources need to be pruned for this AddonsLayer
func (a KubectlLayerApplier) PruneIsRequired(layer layers.Layer) (pruneRequired bool, pruneHrs []*helmopv1.HelmRelease, err error) {
	sourceHrs, err := a.getSourceResources(layer)
	if err != nil {
		return false, nil, err
	}

	hrs := map[string]*helmopv1.HelmRelease{}
	for _, hr := range sourceHrs {
		hrs[getLabel(hr)] = hr
	}

	clusterHrs, err := a.getHelmReleases(layer)
	if err != nil {
		return false, nil, err
	}

	pruneRequired = false
	pruneHrs = []*helmopv1.HelmRelease{}

	for _, hr := range clusterHrs {
		_, ok := hrs[getLabel(hr)]
		if !ok {
			// this resource exists on the cluster but not in the source directory
			a.logInfo("pruned HelmRelease for AddonsLayer in KubeAPI but not in source directory", layer, "pruneResource", hr)
			pruneRequired = true
			pruneHrs = append(pruneHrs, hr)
		}
	}

	return pruneRequired, pruneHrs, nil
}

// ApplyIsRequired returns true if any resources need to be applied for this AddonsLayer
func (a KubectlLayerApplier) ApplyIsRequired(layer layers.Layer) (applyIsRequired bool, err error) {
	sourceHrs, err := a.getSourceResources(layer)
	if err != nil {
		return false, err
	}

	clusterHrs, err := a.getHelmReleases(layer)
	if err != nil {
		return false, err
	}

	hrs := map[string]*helmopv1.HelmRelease{}
	for _, hr := range clusterHrs {
		hrs[getLabel(hr)] = hr
	}

	// Check for any missing resources first.  This is the fastest and easiest check.
	for _, source := range sourceHrs {
		_, ok := hrs[getLabel(source)]
		if !ok {
			// this resource exists in the source directory but not on the cluster
			a.logInfo("found new HelmRelease in AddonsLayer source directory", layer, "newHelmRelease", source.Spec)
			return true, nil
		}
	}

	// Compare each HelmRelease source spec to the spec of the found HelmRelease on the cluster
	for _, source := range sourceHrs {
		found, _ := hrs[getLabel(source)]
		if a.sourceHasChanged(layer, source, found) {
			return true, nil
		}
	}
	return false, nil
}

func (a KubectlLayerApplier) sourceHasChanged(layer layers.Layer, source, found *helmopv1.HelmRelease) (changed bool) {
	a.logTrace("comparing HelmRelease source to KubeAPI resource", layer, "source", source.Spec, "found", found.Spec)
	if !reflect.DeepEqual(source.Spec, found.Spec) || !reflect.DeepEqual(source.ObjectMeta.Labels, found.ObjectMeta.Labels) {
		// this resource source spec does not match the resource spec on the cluster
		a.logInfo("found spec change for HelmRelease in AddonsLayer source directory", layer, "resource", getLabel(source), "source", source.Spec, "found", found.Spec)
		return true
	}
	sourceLabels := source.ObjectMeta.Labels
	foundLabels := found.ObjectMeta.Labels
	if !reflect.DeepEqual(sourceLabels, foundLabels) {
		// this resource source labels do not match the resource labels on the cluster
		a.logInfo("found label change for HelmRelease in AddonsLayer source directory", layer, "resource", getLabel(source), "source", sourceLabels, "found", foundLabels)
		return true
	}
	return false
}

// ApplyWasSuccessful returns true if all of the resources in this AddonsLayer are in the Success phase
func (a KubectlLayerApplier) ApplyWasSuccessful(layer layers.Layer) (applyIsRequired bool, err error) {
	clusterHrs, err := a.getHelmReleases(layer)
	if err != nil {
		return false, err
	}

	for _, hr := range clusterHrs {
		if hr.Status.Phase == helmopv1.HelmReleasePhaseSucceeded {
			a.logDebug("unsuccessful HelmRelease for AddonsLayer", layer, "resource", hr)
			return false, nil
		}
	}

	return true, nil
}
