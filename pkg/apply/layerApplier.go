//Package apply applies Hel Releases
//go:generate mockgen -destination=../mocks/apply/mockLayerApplier.go -package=apply -source=layerApplier.go . LayerApplier
package apply

/*
To generate mock code for the LayerApplier run 'go generate ./...' from the project root directory.
*/
import (
	"context"
	"fmt"
	"os"
	"reflect"

	helmctlv2 "github.com/fluxcd/helm-controller/api/v2alpha1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/fidelity/kraan/pkg/internal/kubectl"
	"github.com/fidelity/kraan/pkg/layers"
	"github.com/fidelity/kraan/pkg/utils"
)

var (
	ownerLabel     string                                            = "kraan/owner"
	newKubectlFunc func(logger logr.Logger) (kubectl.Kubectl, error) = kubectl.NewKubectl
)

// LayerApplier defines methods for managing the Addons within an AddonLayer in a cluster.
type LayerApplier interface {
	Apply(ctx context.Context, layer layers.Layer) (err error)
	Prune(ctx context.Context, layer layers.Layer, pruneHrs []*helmctlv2.HelmRelease) (err error)
	PruneIsRequired(ctx context.Context, layer layers.Layer) (pruneRequired bool, pruneHrs []*helmctlv2.HelmRelease, err error)
	ApplyIsRequired(ctx context.Context, layer layers.Layer) (applyIsRequired bool, err error)
	ApplyWasSuccessful(ctx context.Context, layer layers.Layer) (applyIsRequired bool, err error)
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
		return nil, fmt.Errorf("unable to create a Kubectl provider for KubectlLayerApplier: %w", err)
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
	a.getLog(layer).V(level).Info(msg, append(keysAndValues, "sourcePath", layer.GetSourcePath())...)
}

func (a KubectlLayerApplier) logError(err error, msg string, layer layers.Layer, keysAndValues ...interface{}) {
	a.getLog(layer).Error(err, msg, append(keysAndValues, "sourcePath", layer.GetSourcePath())...)
}

func (a KubectlLayerApplier) logInfo(msg string, layer layers.Layer, keysAndValues ...interface{}) {
	a.log(0, msg, layer, keysAndValues...)
}

func (a KubectlLayerApplier) logDebug(msg string, layer layers.Layer, keysAndValues ...interface{}) {
	a.log(1, msg, layer, keysAndValues...)
}

func (a KubectlLayerApplier) logTrace(msg string, layer layers.Layer, keysAndValues ...interface{}) {
	a.log(7, msg, layer, keysAndValues...)
}

/*
func (a KubectlLayerApplier) logErrors(errz []error, layer layers.Layer) {
	for _, err := range errz {
		a.logError(err, "error while applying layer", layer)
	}
}
*/
func getLabel(hr metav1.ObjectMeta) string {
	return fmt.Sprintf("%s/%s", hr.GetNamespace(), hr.GetName())
}

func removeResourceVersion(obj runtime.Object) string {
	mobj, ok := (obj).(metav1.Object)
	if !ok {
		return fmt.Sprintf("unable to convert runtime.Object to meta.Object")
	}
	mobj.SetResourceVersion("")
	return fmt.Sprintf("%s/%s", mobj.GetNamespace(), mobj.GetName())
}

func getObjLabel(obj runtime.Object) string {
	mobj, ok := (obj).(metav1.Object)
	if !ok {
		return fmt.Sprintf("unable to convert runtime.Object to meta.Object")
	}
	return fmt.Sprintf("%s/%s", mobj.GetNamespace(), mobj.GetName())
}

func (a KubectlLayerApplier) decodeHelmReleases(layer layers.Layer, objs []runtime.Object) (hrs []*helmctlv2.HelmRelease, err error) {
	for i, obj := range objs {
		a.logTrace("checking for HelmRelease type", layer, "object", obj)
		switch obj.(type) {
		case *helmctlv2.HelmRelease:
			hr, ok := obj.(*helmctlv2.HelmRelease)
			if ok {
				a.logTrace("found HelmRelease in Object list", layer, "index", i, "helmRelease", hr)
				hrs = append(hrs, hr)
			} else {
				err = fmt.Errorf("unable to convert runtime.Object to HelmRelease")
				a.logError(err, err.Error(), layer, "runtimeObject", obj)
			}
		default:
			a.logInfo("found Kubernetes object in Object list", layer, "index", i, "object", obj)
		}
	}
	return hrs, err
}

func (a KubectlLayerApplier) decodeHelmRepos(layer layers.Layer, objs []runtime.Object) (hrs []*sourcev1.HelmRepository, err error) {
	for i, obj := range objs {
		a.logTrace("checking for HelmRepository type", layer, "object", obj)
		switch obj.(type) {
		case *sourcev1.HelmRepository:
			hr, ok := obj.(*sourcev1.HelmRepository)
			if ok {
				a.logTrace("found HelmRelease in Object list", layer, "index", i, "helmRelease", hr)
				hrs = append(hrs, hr)
			} else {
				err = fmt.Errorf("unable to convert runtime.Object to HelmRelease")
				a.logError(err, err.Error(), layer, "runtimeObject", obj)
			}
		default:
			a.logInfo("found Kubernetes object in Object list", layer, "index", i, "object", obj)
		}
	}
	return hrs, err
}

func (a KubectlLayerApplier) decodeAddons(layer layers.Layer,
	json []byte) (objs []runtime.Object, err error) {
	// TODO - should probably trace log the json before we try to decode it.
	a.logTrace("decoding JSON output from kubectl", layer, "output", string(json))

	// dez := a.scheme.Codecs.UniversalDeserializer()
	dez := serializer.NewCodecFactory(a.scheme).UniversalDeserializer()

	obj, gvk, err := dez.Decode(json, nil, nil)
	if err != nil {
		a.logError(err, "unable to parse JSON output from kubectl", layer, "output", string(json))
		return nil, err
	}

	a.logTrace("decoded JSON output", layer, "groupVersionKind", gvk, "object", utils.LogJSON(obj))

	switch obj.(type) {
	case *corev1.List:
		a.logTrace("decoded raw object List from kubectl output", layer, "groupVersionKind", gvk, "list", utils.LogJSON(obj))
		return a.decodeList(layer, obj.(*corev1.List), &dez)
	default:
		/*msg := "decoded kubectl output was not a HelmRelease or List"
		err = fmt.Errorf(msg)
		a.logError(err, msg, layer, "output", string(json), "groupVersionKind", gvk, "object", obj)*/
		return []runtime.Object{obj}, nil
	}
}

func (a KubectlLayerApplier) addOwnerRefs(layer layers.Layer, objs []runtime.Object) error {
	for i, robj := range objs {
		obj, ok := robj.(metav1.Object)
		if !ok {
			err := fmt.Errorf("unable to convert runtime.Object to meta.Object")
			a.logError(err, err.Error(), layer, "runtimeObject", robj)
			return err
		}
		a.logTrace("Adding owner ref to resource for AddonsLayer", layer, "index", i, "obj", obj)
		err := controllerutil.SetControllerReference(layer.GetAddonsLayer(), obj, a.scheme)
		if err != nil {
			// could not apply owner ref for object
			return fmt.Errorf("unable to apply owner reference for AddonsLayer '%s' to resource '%s': %w", layer.GetName(), getObjLabel(robj), err)
		}
		labels := obj.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		labels[ownerLabel] = layer.GetName()
		obj.SetLabels(labels)
	}
	return nil
}

func (a KubectlLayerApplier) getHelmReleases(ctx context.Context, layer layers.Layer) (foundHrs []*helmctlv2.HelmRelease, err error) {
	hrList := &helmctlv2.HelmReleaseList{}
	err = a.client.List(ctx, hrList, client.MatchingFields{".owner": layer.GetName()})
	if err != nil {
		return nil, fmt.Errorf("unable to list HelmRelease resources owned by '%s': %w", layer.GetName(), err)
	}
	for _, hr := range hrList.Items {
		foundHrs = append(foundHrs, hr.DeepCopy())
	}
	return foundHrs, nil
}

func (a KubectlLayerApplier) applyObjects(ctx context.Context, layer layers.Layer, objs []runtime.Object) error {
	a.logDebug("To be applied resources for AddonsLayer", layer, "objects", utils.LogJSON(objs))
	for i, obj := range objs {
		a.logDebug("Applying resources for AddonsLayer", layer, "index", i, "object", utils.LogJSON(obj))
		/*
			res, err := controllerutil.CreateOrUpdate(ctx, a.client, obj, func() error {
				fmt.Fprintln(os.Stderr, "mutate")
				return nil
			})
		*/
		err := a.applyObject(ctx, layer, obj)
		if err != nil {
			return fmt.Errorf("unable to apply layer resource: %w", err)
		}
		a.logInfo("resource successfully applied", layer, "resource", getObjLabel(obj))
	}
	return nil
}

func (a KubectlLayerApplier) getHelmRepos(ctx context.Context, layer layers.Layer) (foundHrs []*sourcev1.HelmRepository, err error) {
	hrList := &sourcev1.HelmRepositoryList{}
	err = a.client.List(ctx, hrList, client.MatchingFields{".owner": layer.GetName()})
	if err != nil {
		return nil, fmt.Errorf("unable to list HelmRepos resources owned by '%s': %w", layer.GetName(), err)
	}
	for _, hr := range hrList.Items {
		foundHrs = append(foundHrs, hr.DeepCopy())
	}
	return foundHrs, nil
}

func (a KubectlLayerApplier) isObjectPresent(ctx context.Context, layer layers.Layer, obj runtime.Object) (bool, error) {
	key, err := client.ObjectKeyFromObject(obj)
	if err != nil {
		return false, fmt.Errorf("unable to get an ObjectKey '%s': %w", getObjLabel(obj), err)
	}
	//existing := obj.DeepCopyObject()
	err = a.client.Get(ctx, key, obj)
	if err != nil {
		if errors.IsNotFound(err) {
			removeResourceVersion(obj)
			a.logDebug("existing object not found", layer, "key", key)
			return false, nil
		}
		return false, fmt.Errorf("failed to Get object '%s': %w", key, err)
	}
	a.logDebug("existing object found", layer, "key", key)
	return true, nil
}

func (a KubectlLayerApplier) applyObject(ctx context.Context, layer layers.Layer, obj runtime.Object) error {
	a.logDebug("Applying object for AddonsLayer", layer, "object", obj)
	present, err := a.isObjectPresent(ctx, layer, obj)
	if err != nil {
		return fmt.Errorf("unable to determine if object '%s' is present on the target cluster: %w", getObjLabel(obj), err)
	}
	if !present {
		// object does not exist, create resource
		err = a.client.Create(ctx, obj, &client.CreateOptions{})
		if err != nil {
			return fmt.Errorf("unable to Create Object '%s' on the target cluster: %w", getObjLabel(obj), err)
		}
	} else {
		// Object exists, update resource
		err = a.client.Update(ctx, obj, &client.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("unable to Update object '%s' on the target cluster: %w", getObjLabel(obj), err)
		}
	}

	return nil
}

func (a KubectlLayerApplier) decodeList(layer layers.Layer,
	raws *corev1.List, dez *runtime.Decoder) (objs []runtime.Object, err error) {
	dec := *dez

	a.logTrace("decoding list of raw JSON items", layer, "length", len(raws.Items))

	for i, raw := range raws.Items {
		obj, gvk, decodeErr := dec.Decode(raw.Raw, nil, nil)
		if decodeErr != nil {
			err = fmt.Errorf("could not decode JSON to a runtime.Object: %w", decodeErr)
			a.logError(err, err.Error(), layer, "rawJSON", string(raw.Raw))
		}
		a.logInfo("decoded Kubernetes object from kubectl output list",
			layer, "index", i, "groupVersionKind", gvk, "object", obj)
		objs = append(objs, obj)
	}
	return objs, err
}

func (a KubectlLayerApplier) checkSourcePath(layer layers.Layer) (sourceDir string, err error) {
	a.logTrace("Checking layer source directory", layer)
	sourceDir = layer.GetSourcePath()
	info, err := os.Stat(sourceDir)
	if os.IsNotExist(err) {
		a.logDebug("source directory not found", layer)
		return sourceDir, fmt.Errorf("source directory (%s) not found for AddonsLayer %s",
			sourceDir, layer.GetName())
	}
	if os.IsPermission(err) {
		a.logDebug("source directory read permission denied", layer)
		return sourceDir, fmt.Errorf("read permission denied to source directory (%s) for AddonsLayer %s",
			sourceDir, layer.GetName())
	}
	if err != nil {
		a.logError(err, "error while checking source directory", layer)
		return sourceDir, fmt.Errorf("error while checking source directory (%s) for AddonsLayer %s: %w",
			sourceDir, layer.GetName(), err)
	}
	if !info.IsDir() {
		// I'm not sure if this is an error, but I thought I should detect and log it
		a.logInfo("source path is not a directory", layer)
	}
	return sourceDir, nil
}

func (a KubectlLayerApplier) getSourceResources(layer layers.Layer) (objs []runtime.Object, err error) {
	sourceDir, err := a.checkSourcePath(layer)
	if err != nil {
		return nil, err
	}
	dirSlash := fmt.Sprintf("%s/", sourceDir)
	output, err := a.kubectl.Apply(dirSlash).WithLogger(layer.GetLogger()).DryRun()
	if err != nil {
		return nil, fmt.Errorf("error from kubectl while parsing source directory (%s) for AddonsLayer %s: %w",
			sourceDir, layer.GetName(), err)
	}

	objs, err = a.decodeAddons(layer, output)
	if err != nil {
		return nil, err
	}

	err = a.addOwnerRefs(layer, objs)
	if err != nil {
		return nil, err
	}

	return objs, nil
}

func (a KubectlLayerApplier) getSourceHelmReleases(layer layers.Layer) (hrs []*helmctlv2.HelmRelease, err error) {
	objs, err := a.getSourceResources(layer)
	if err != nil {
		return nil, err
	}

	hrs, err = a.decodeHelmReleases(layer, objs)
	if err != nil {
		return nil, err
	}

	return hrs, nil
}

func (a KubectlLayerApplier) getSourceHelmRepos(layer layers.Layer) (hrs []*sourcev1.HelmRepository, err error) {
	objs, err := a.getSourceResources(layer)
	if err != nil {
		return nil, err
	}

	hrs, err = a.decodeHelmRepos(layer, objs)
	if err != nil {
		return nil, err
	}

	return hrs, nil
}

// Apply an AddonLayer to the cluster.
func (a KubectlLayerApplier) Apply(ctx context.Context, layer layers.Layer) (err error) {
	a.logInfo("Applying AddonsLayer", layer)

	hrs, err := a.getSourceResources(layer)
	if err != nil {
		return err
	}

	err = a.applyObjects(ctx, layer, hrs)
	if err != nil {
		return err
	}
	return nil
}

// Prune the AddonsLayer by removing the Addons found in the cluster that have since been removed from the Layer.
func (a KubectlLayerApplier) Prune(ctx context.Context, layer layers.Layer, pruneHrs []*helmctlv2.HelmRelease) (err error) {
	for _, hr := range pruneHrs {
		err := a.client.Delete(ctx, hr, client.PropagationPolicy(metav1.DeletePropagationBackground))
		if err != nil {
			return fmt.Errorf("unable to delete HelmRelease '%s' for AddonsLayer '%s': %w", getLabel(hr.ObjectMeta), layer.GetName(), err)
		}
	}
	return nil
}

// PruneIsRequired returns true if any resources need to be pruned for this AddonsLayer
func (a KubectlLayerApplier) PruneIsRequired(ctx context.Context, layer layers.Layer) (pruneRequired bool, pruneHrs []*helmctlv2.HelmRelease, err error) {
	sourceHrs, err := a.getSourceHelmReleases(layer)
	if err != nil {
		return false, nil, err
	}

	hrs := map[string]*helmctlv2.HelmRelease{}
	for _, hr := range sourceHrs {
		hrs[getLabel(hr.ObjectMeta)] = hr
	}

	clusterHrs, err := a.getHelmReleases(ctx, layer)
	if err != nil {
		return false, nil, err
	}

	pruneRequired = false
	pruneHrs = []*helmctlv2.HelmRelease{}

	for _, hr := range clusterHrs {
		_, ok := hrs[getLabel(hr.ObjectMeta)]
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
func (a KubectlLayerApplier) ApplyIsRequired(ctx context.Context, layer layers.Layer) (applyIsRequired bool, err error) {
	sourceHrs, err := a.getSourceHelmReleases(layer)
	if err != nil {
		return false, err
	}

	clusterHrs, err := a.getHelmReleases(ctx, layer)
	if err != nil {
		return false, err
	}

	hrs := map[string]*helmctlv2.HelmRelease{}
	for _, hr := range clusterHrs {
		hrs[getLabel(hr.ObjectMeta)] = hr
	}

	// Check for any missing resources first.  This is the fastest and easiest check.
	for _, source := range sourceHrs {
		_, ok := hrs[getLabel(source.ObjectMeta)]
		if !ok {
			// this resource exists in the source directory but not on the cluster
			a.logInfo("found new HelmRelease in AddonsLayer source directory", layer, "newHelmRelease", source.Spec)
			return true, nil
		}
	}

	// Compare each HelmRelease source spec to the spec of the found HelmRelease on the cluster
	for _, source := range sourceHrs {
		found := hrs[getLabel(source.ObjectMeta)]
		if a.sourceHasReleaseChanged(layer, source, found) {
			return true, nil
		}
	}
	return a.helmReposApplyRequired(ctx, layer)
}

// ApplyIsRequired returns true if any resources need to be applied for this AddonsLayer
func (a KubectlLayerApplier) helmReposApplyRequired(ctx context.Context, layer layers.Layer) (applyIsRequired bool, err error) {
	sourceHrs, err := a.getSourceHelmRepos(layer)
	if err != nil {
		return false, err
	}

	clusterHrs, err := a.getHelmRepos(ctx, layer)
	if err != nil {
		return false, err
	}

	hrs := map[string]*sourcev1.HelmRepository{}
	for _, hr := range clusterHrs {
		hrs[getLabel(hr.ObjectMeta)] = hr
	}

	// Check for any missing resources first.  This is the fastest and easiest check.
	for _, source := range sourceHrs {
		_, ok := hrs[getLabel(source.ObjectMeta)]
		if !ok {
			// this resource exists in the source directory but not on the cluster
			a.logInfo("found new HelmRepository in AddonsLayer source directory", layer, "new HelmRepository", source.Spec)
			return true, nil
		}
	}

	// Compare each HelmRelease source spec to the spec of the found HelmRelease on the cluster
	for _, source := range sourceHrs {
		found := hrs[getLabel(source.ObjectMeta)]
		if a.sourceHasRepoChanged(layer, source, found) {
			return true, nil
		}
	}
	return false, nil
}

func (a KubectlLayerApplier) sourceHasReleaseChanged(layer layers.Layer, source, found *helmctlv2.HelmRelease) (changed bool) {
	if !utils.CompareAsJSON(source.Spec, found.Spec) {
		a.logInfo("found spec change for HelmRelease in AddonsLayer source directory", layer, "resource", getLabel(source.ObjectMeta),
			"source", source.Spec, "found", found.Spec, "diff", cmp.Diff(source.Spec, found.Spec))
		return true
	}
	if !reflect.DeepEqual(source.ObjectMeta.Labels, found.ObjectMeta.Labels) {
		// this resource source spec does not match the resource spec on the cluster
		a.logInfo("found label change for HelmRelease in AddonsLayer source directory", layer, "resource", getLabel(source.ObjectMeta),
			"label source", source.ObjectMeta.Labels, "label found", found.ObjectMeta.Labels)
		return true
	}
	a.logTrace("found no changes for HelmRelease in AddonsLayer source directory", layer)
	return false
}

func (a KubectlLayerApplier) sourceHasRepoChanged(layer layers.Layer, source, found *sourcev1.HelmRepository) (changed bool) {
	if !utils.CompareAsJSON(source.Spec, found.Spec) {
		a.logInfo("found spec change for HelmRepository in AddonsLayer source directory", layer, "resource", getLabel(source.ObjectMeta),
			"source", source.Spec, "found", found.Spec, "diff", cmp.Diff(source.Spec, found.Spec))
		return true
	}
	if !reflect.DeepEqual(source.ObjectMeta.Labels, found.ObjectMeta.Labels) {
		// this resource source spec does not match the resource spec on the cluster
		a.logInfo("found label change for HelmRepository in AddonsLayer source directory", layer, "resource", getLabel(source.ObjectMeta),
			"label source", source.ObjectMeta.Labels, "label found", found.ObjectMeta.Labels)
		return true
	}
	a.logTrace("found no change for HelmRepositories in AddonsLayer source directory", layer)
	return false
}

// ApplyWasSuccessful returns true if all of the resources in this AddonsLayer are in the Success phase
func (a KubectlLayerApplier) ApplyWasSuccessful(ctx context.Context, layer layers.Layer) (applyIsRequired bool, err error) {
	clusterHrs, err := a.getHelmReleases(ctx, layer)
	if err != nil {
		return false, err
	}

	for _, hr := range clusterHrs {
		if !a.CheckHR(*hr, layer) {
			a.logDebug("unsuccessful HelmRelease for AddonsLayer", layer, "resource", hr)
			return false, nil
		}
	}

	return true, nil
}

func (a KubectlLayerApplier) CheckHR(hr helmctlv2.HelmRelease, layer layers.Layer) bool {
	a.logDebug("Check HelmRelease for AddonsLayer", layer, "resource", hr)
	cond := helmctlv2.GetHelmReleaseCondition(hr, "Ready")
	if cond == nil {
		a.logDebug("HelmRelease for AddonsLayer not installed", layer, "resource", hr)
		return false
	}
	// "reason": "ReconciliationSucceeded",
	//       "message": "release reconciliation succeeded"

	a.logDebug("HelmRelease for AddonsLayer installed", layer, "resource", hr, "condition", cond)
	return cond.Status == "True"
}
