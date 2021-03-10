//Package apply applies Hel Releases
//go:generate mockgen -destination=../mocks/apply/mockLayerApplier.go -package=mocks -source=layerApplier.go . LayerApplier
package apply

/*
To generate mock code for the LayerApplier run 'go generate ./...' from the project root directory.
*/
import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	helmctlv2 "github.com/fluxcd/helm-controller/api/v2beta1"
	fluxmeta "github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kraanv1alpha1 "github.com/fidelity/kraan/api/v1alpha1"
	"github.com/fidelity/kraan/pkg/internal/kubectl"
	"github.com/fidelity/kraan/pkg/layers"
	"github.com/fidelity/kraan/pkg/logging"
)

const (
	orphanedLabel = "orphaned"
)

var (
	ownerLabel     string                                            = "kraan/layer"
	newKubectlFunc func(logger logr.Logger) (kubectl.Kubectl, error) = kubectl.NewKubectl
)

// LayerApplier defines methods for managing the Addons within an AddonLayer in a cluster.
type LayerApplier interface {
	Apply(ctx context.Context, layer layers.Layer) (err error)
	Prune(ctx context.Context, layer layers.Layer, pruneHrs []*helmctlv2.HelmRelease) (err error)
	PruneIsRequired(ctx context.Context, layer layers.Layer) (pruneRequired bool, pruneHrs []*helmctlv2.HelmRelease, err error)
	ApplyIsRequired(ctx context.Context, layer layers.Layer) (applyIsRequired bool, err error)
	ApplyWasSuccessful(ctx context.Context, layer layers.Layer) (applyIsRequired bool, hrName string, err error)
	GetResources(ctx context.Context, layer layers.Layer) (resources []kraanv1alpha1.Resource, err error)
	GetSourceAndClusterHelmReleases(ctx context.Context, layer layers.Layer) (sourceHrs, clusterHrs map[string]*helmctlv2.HelmRelease, err error)
	Orphan(ctx context.Context, layer layers.Layer, hr *helmctlv2.HelmRelease) (bool, error)
	GetOrphanedHelmReleases(ctx context.Context, layer layers.Layer) (foundHrs map[string]*helmctlv2.HelmRelease, err error)
	Adopt(ctx context.Context, layer layers.Layer, hr *helmctlv2.HelmRelease) error
	addOwnerRefs(layer layers.Layer, objs []runtime.Object) error
	orphanLabel(ctx context.Context, hr *helmctlv2.HelmRelease) (*metav1.Time, error)
	GetHelmReleases(ctx context.Context, layer layers.Layer) (foundHrs map[string]*helmctlv2.HelmRelease, err error)
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
		return nil, errors.WithMessagef(err, "%s - failed to create a Kubectl provider for KubectlLayerApplier", logging.CallerStr(logging.Me))
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
	a.getLog(layer).V(level).Info(msg, append(keysAndValues, append(logging.GetFunctionAndSource(logging.MyCaller+2), "sourcePath", layer.GetSourcePath())...)...)
}

func (a KubectlLayerApplier) logError(err error, msg string, layer layers.Layer, keysAndValues ...interface{}) {
	a.getLog(layer).Error(err, msg, append(keysAndValues, append(logging.GetFunctionAndSource(logging.MyCaller+2), "sourcePath", layer.GetSourcePath())...)...)
}

func (a KubectlLayerApplier) logInfo(msg string, layer layers.Layer, keysAndValues ...interface{}) {
	a.log(0, msg, layer, keysAndValues...)
}

func (a KubectlLayerApplier) logDebug(msg string, layer layers.Layer, keysAndValues ...interface{}) {
	a.log(1, msg, layer, keysAndValues...)
}

func (a KubectlLayerApplier) logTrace(msg string, layer layers.Layer, keysAndValues ...interface{}) {
	a.log(3, msg, layer, keysAndValues...)
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

func removeResourceVersion(obj runtime.Object) {
	mobj, ok := (obj).(metav1.Object)
	if !ok {
		return
	}
	mobj.SetResourceVersion("")
	return
}

func getObjLabel(obj runtime.Object) string {
	mobj, ok := (obj).(metav1.Object)
	if !ok {
		return fmt.Sprintf("failed to convert runtime.Object to meta.Object")
	}
	return fmt.Sprintf("%s/%s", mobj.GetNamespace(), mobj.GetName())
}

// GetOrphanedHelmReleases returns a map of HelmReleases that are labeled as orphaned and not owned by layer
func (a KubectlLayerApplier) GetOrphanedHelmReleases(ctx context.Context, layer layers.Layer) (foundHrs map[string]*helmctlv2.HelmRelease, err error) {
	logging.TraceCall(a.getLog(layer))
	defer logging.TraceExit(a.getLog(layer))

	hrList := &helmctlv2.HelmReleaseList{}
	var labels client.HasLabels = []string{orphanedLabel}
	listOptions := &client.ListOptions{}
	labels.ApplyToList(listOptions)

	err = a.client.List(ctx, hrList, listOptions)
	if err != nil {
		return nil, errors.Wrapf(err, "%s - failed to list orphaned HelmRelease resources '%s'", logging.CallerStr(logging.Me), layer.GetName())
	}

	foundHrs = map[string]*helmctlv2.HelmRelease{}
	for _, hr := range hrList.Items {
		if layer.GetName() != layerOwner(&hr) { // nolint: scopelint // ok
			foundHrs[getLabel(hr.ObjectMeta)] = hr.DeepCopy()
		}
	}
	return foundHrs, nil
}
func (a KubectlLayerApplier) decodeHelmReleases(layer layers.Layer, objs []runtime.Object) (hrs []*helmctlv2.HelmRelease, err error) {
	logging.TraceCall(a.getLog(layer))
	defer logging.TraceExit(a.getLog(layer))
	for _, obj := range objs {
		a.logDebug("checking for HelmRelease type", layer, logging.GetObjNamespaceName(obj)...)
		switch obj.(type) {
		case *helmctlv2.HelmRelease:
			hr, ok := obj.(*helmctlv2.HelmRelease)
			if ok {
				a.logDebug("found HelmRelease in Object list", layer, append(logging.GetObjKindNamespaceName(obj), "helmRelease", hr)...)
				hrs = append(hrs, hr)
			} else {
				err = fmt.Errorf("failed to convert runtime.Object to HelmRelease")
				a.logError(err, err.Error(), layer, logging.GetObjNamespaceName(obj)...)
			}
		default:
			a.logTrace("found Kubernetes object in Object list", layer, logging.GetObjKindNamespaceName(obj)...)
		}
	}
	return hrs, err
}

func (a KubectlLayerApplier) decodeHelmRepos(layer layers.Layer, objs []runtime.Object) (hrs []*sourcev1.HelmRepository, err error) {
	logging.TraceCall(a.getLog(layer))
	defer logging.TraceExit(a.getLog(layer))
	for _, obj := range objs {
		a.logTrace("checking for HelmRepository type", layer, "object", obj)
		switch obj.(type) {
		case *sourcev1.HelmRepository:
			hr, ok := obj.(*sourcev1.HelmRepository)
			if ok {
				a.logDebug("found HelmRepository in Object list", layer, append(logging.GetObjKindNamespaceName(obj), "helmRelease", hr)...)
				hrs = append(hrs, hr)
			} else {
				err = fmt.Errorf("failed to convert runtime.Object to HelmRepository")
				a.logError(err, err.Error(), layer, logging.GetObjKindNamespaceName(obj)...)
			}
		default:
			a.logDebug("found Kubernetes object in Object list", layer, logging.GetObjNamespaceName(obj)...)
		}
	}
	return hrs, err
}

func (a KubectlLayerApplier) decodeAddons(layer layers.Layer,
	json []byte) (objs []runtime.Object, err error) {
	logging.TraceCall(a.getLog(layer))
	defer logging.TraceExit(a.getLog(layer))
	// TODO - should probably trace log the json before we try to decode it.
	a.logTrace("decoding JSON output from kubectl", layer, "output", string(json))

	// dez := a.scheme.Codecs.UniversalDeserializer()
	dez := serializer.NewCodecFactory(a.scheme).UniversalDeserializer()

	obj, gvk, err := dez.Decode(json, nil, nil)
	if err != nil {
		a.logError(err, "unable to parse JSON output from kubectl", layer, "output", string(json))
		return nil, errors.Wrapf(err, "%s - failed to decode json", logging.CallerStr(logging.Me))
	}

	a.logTrace("decoded JSON output", layer, "groupVersionKind", gvk, "object", logging.LogJSON(obj))

	switch obj.(type) {
	case *corev1.List:
		a.logTrace("decoded raw object List from kubectl output", layer, "list", logging.LogJSON(obj))
		return a.decodeList(layer, obj.(*corev1.List), &dez)
	default:
		/*msg := "decoded kubectl output was not a HelmRelease or List"
		err = fmt.Errorf(msg)
		a.logError(err, msg, layer, "output", string(json), "groupVersionKind", gvk, "object", obj)*/
		return []runtime.Object{obj}, nil
	}
}

func (a KubectlLayerApplier) addOwnerRefs(layer layers.Layer, objs []runtime.Object) error {
	logging.TraceCall(a.getLog(layer))
	defer logging.TraceExit(a.getLog(layer))
	for _, robj := range objs {
		obj, ok := robj.(metav1.Object)
		if !ok {
			err := fmt.Errorf("failed to convert runtime.Object to meta.Object")
			a.logError(err, err.Error(), layer, logging.GetObjKindNamespaceName(robj)...)
			return err
		}

		owningLayer := layerOwner(obj)
		if len(owningLayer) > 0 && owningLayer != layer.GetName() {
			a.logDebug("resource already owned by another AddonsLayer", layer, logging.GetObjKindNamespaceName(robj)...)
			if len(labelValue(orphanedLabel, &obj)) > 0 {
				return nil
			}
			return fmt.Errorf("%s - HelmRelease: %s, also included in layer: %s", logging.CallerStr(logging.Me), getObjLabel(robj), labelValue(ownerLabel, &obj))
		}

		a.logDebug("Adding owner ref to resource for AddonsLayer", layer, logging.GetObjKindNamespaceName(robj)...)
		err := controllerutil.SetControllerReference(layer.GetAddonsLayer(), obj, a.scheme)
		if err != nil {
			// could not apply owner ref for object
			return errors.Wrapf(err, "%s - failed to apply owner reference to: %s", logging.CallerStr(logging.Me), getObjLabel(robj))
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

func (a KubectlLayerApplier) GetHelmReleases(ctx context.Context, layer layers.Layer) (foundHrs map[string]*helmctlv2.HelmRelease, err error) {
	logging.TraceCall(a.getLog(layer))
	defer logging.TraceExit(a.getLog(layer))
	hrList := &helmctlv2.HelmReleaseList{}
	err = a.client.List(ctx, hrList, client.MatchingFields{".owner": layer.GetName()})
	if err != nil {
		return nil, errors.Wrapf(err, "%s - failed to list HelmRelease resources owned by '%s'", logging.CallerStr(logging.Me), layer.GetName())
	}
	foundHrs = map[string]*helmctlv2.HelmRelease{}
	for _, hr := range hrList.Items {
		foundHrs[getLabel(hr.ObjectMeta)] = hr.DeepCopy()
	}
	return foundHrs, nil
}

func (a KubectlLayerApplier) applyHelmReleaseObjects(ctx context.Context, layer layers.Layer, objs map[string]*helmctlv2.HelmRelease) error {
	logging.TraceCall(a.getLog(layer))
	defer logging.TraceExit(a.getLog(layer))
	a.logTrace("helm release resources be applied", layer, "objects", logging.LogJSON(objs))
	for _, obj := range objs {
		a.logTrace("applying helm release resource", layer, "object", logging.LogJSON(obj))
		err := a.applyObject(ctx, layer, obj)
		if err != nil {
			return errors.Wrapf(err, "%s - failed to apply layer helm release resource", logging.CallerStr(logging.Me))
		}
		a.logDebug("helm release resource successfully applied", layer, logging.GetObjKindNamespaceName(obj)...)
	}
	return nil
}

func (a KubectlLayerApplier) applyHelmRepoObjects(ctx context.Context, layer layers.Layer, objs []*sourcev1.HelmRepository) error {
	logging.TraceCall(a.getLog(layer))
	defer logging.TraceExit(a.getLog(layer))
	a.logTrace("helm repository resources be applied", layer, "objects", logging.LogJSON(objs))
	for _, obj := range objs {
		a.logTrace("applying helm repositoryresource", layer, "object", logging.LogJSON(obj))
		err := a.applyObject(ctx, layer, obj)
		if err != nil {
			return errors.Wrapf(err, "%s - failed to apply layer helm repository resource", logging.CallerStr(logging.Me))
		}
		a.logDebug("helm repository resource successfully applied", layer, logging.GetObjKindNamespaceName(obj)...)
	}
	return nil
}

/*
func (a KubectlLayerApplier) applyObjects(ctx context.Context, layer layers.Layer, objs []runtime.Object) error {
	logging.TraceCall(a.getLog(layer))
	defer logging.TraceExit(a.getLog(layer))
	a.logTrace("resources be applied", layer, "objects", logging.LogJSON(objs))
	for _, obj := range objs {
		a.logTrace("applying resource", layer, "object", logging.LogJSON(obj))
		err := a.applyObject(ctx, layer, obj)
		if err != nil {
			return errors.Wrapf(err, "%s - failed to apply layer resources", logging.CallerStr(logging.Me))
		}
		a.logDebug("resource successfully applied", layer, logging.GetObjKindNamespaceName(obj)...)
	}
	return nil
}
*/

func (a KubectlLayerApplier) getHelmRepos(ctx context.Context, layer layers.Layer) (foundHrs []*sourcev1.HelmRepository, err error) {
	logging.TraceCall(a.getLog(layer))
	defer logging.TraceExit(a.getLog(layer))
	hrList := &sourcev1.HelmRepositoryList{}
	err = a.client.List(ctx, hrList, client.MatchingFields{".owner": layer.GetName()})
	if err != nil {
		return nil, errors.Wrapf(err, "%s - failed to list HelmRepos resources owned by '%s'", logging.CallerStr(logging.Me), layer.GetName())
	}
	for _, hr := range hrList.Items {
		foundHrs = append(foundHrs, hr.DeepCopy())
	}
	return foundHrs, nil
}

func (a KubectlLayerApplier) isObjectPresent(ctx context.Context, layer layers.Layer, obj runtime.Object) (bool, error) {
	logging.TraceCall(a.getLog(layer))
	defer logging.TraceExit(a.getLog(layer))
	key, err := client.ObjectKeyFromObject(obj)
	if err != nil {
		return false, errors.Wrapf(err, "%s - failed to get an ObjectKey '%s'", logging.CallerStr(logging.Me), getObjLabel(obj))
	}
	existing := obj.DeepCopyObject()
	err = a.client.Get(ctx, key, existing)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			removeResourceVersion(obj)
			a.logDebug("existing object not found", layer, logging.GetObjKindNamespaceName(obj)...)
			return false, nil
		}
		return false, errors.Wrapf(err, "%s - failed to get an ObjectKey '%s'", logging.CallerStr(logging.Me), key)
	}
	a.logDebug("existing object found", layer, logging.GetObjKindNamespaceName(obj)...)
	return true, nil
}

func (a KubectlLayerApplier) applyObject(ctx context.Context, layer layers.Layer, obj runtime.Object) error {
	logging.TraceCall(a.getLog(layer))
	defer logging.TraceExit(a.getLog(layer))
	a.logDebug("applying object", layer, logging.GetObjKindNamespaceName(obj)...)
	present, err := a.isObjectPresent(ctx, layer, obj)
	if err != nil {
		return errors.WithMessagef(err, "%s - failed to determine if object '%s' is present on the target cluster", logging.CallerStr(logging.Me), getObjLabel(obj))
	}
	if !present {
		// object does not exist, create resource
		err = a.client.Create(ctx, obj, &client.CreateOptions{})
		if err != nil {
			return errors.Wrapf(err, "%s - failed to Create Object '%s' on the target cluster", logging.CallerStr(logging.Me), getObjLabel(obj))
		}
	} else {
		// Object exists, update resource
		err = a.client.Update(ctx, obj, &client.UpdateOptions{})
		if err != nil {
			return errors.Wrapf(err, "%s - failed to Update object '%s' on the target cluster", logging.CallerStr(logging.Me), getObjLabel(obj))
		}
	}

	return nil
}

func (a KubectlLayerApplier) decodeList(layer layers.Layer,
	raws *corev1.List, dez *runtime.Decoder) (objs []runtime.Object, err error) {
	logging.TraceCall(a.getLog(layer))
	defer logging.TraceExit(a.getLog(layer))
	dec := *dez

	a.logTrace("decoding list of raw JSON items", layer, "length", len(raws.Items))

	for _, raw := range raws.Items {
		obj, _, decodeErr := dec.Decode(raw.Raw, nil, nil)
		if decodeErr != nil {
			err = fmt.Errorf("could not decode JSON to a runtime.Object: %w", decodeErr)
			a.logError(err, err.Error(), layer, "rawJSON", string(raw.Raw))
		}
		a.logDebug("decoded Kubernetes object from kubectl output list",
			layer, logging.GetObjKindNamespaceName(obj)...)
		objs = append(objs, obj)
	}
	return objs, err
}

func (a KubectlLayerApplier) checkSourcePath(layer layers.Layer) (sourceDir string, err error) {
	logging.TraceCall(a.getLog(layer))
	defer logging.TraceExit(a.getLog(layer))
	a.logDebug("Checking layer source directory", layer)
	sourceDir = layer.GetSourcePath()
	info, err := os.Stat(sourceDir)
	if os.IsNotExist(err) {
		a.logDebug("source directory not found", layer)
		return sourceDir, errors.Wrapf(err, "source directory (%s) not found for AddonsLayer %s",
			sourceDir, layer.GetName())
	}
	if os.IsPermission(err) {
		a.logDebug("source directory read permission denied", layer)
		return sourceDir, errors.Wrapf(err, "read permission denied to source directory (%s) for AddonsLayer %s",
			sourceDir, layer.GetName())
	}
	if err != nil {
		a.logError(err, "error while checking source directory", layer)
		return sourceDir, errors.Wrapf(err, "%s - failed to check source directory (%s) for AddonsLayer %s",
			logging.CallerStr(logging.Me), sourceDir, layer.GetName())
	}
	if info.IsDir() {
		sourceDir = sourceDir + string(os.PathSeparator)
	} else {
		// I'm not sure if this is an error, but I thought I should detect and log it
		return sourceDir, errors.Wrapf(err, "source path: %s, is not a directory", sourceDir)
	}
	return sourceDir, nil
}

func (a KubectlLayerApplier) doApply(layer layers.Layer, sourceDir string) (output []byte, err error) {
	logging.TraceCall(a.getLog(layer))
	defer logging.TraceExit(a.getLog(layer))
	MaxTries := 5
	for try := 1; try < MaxTries; try++ {
		output, err = a.kubectl.Apply(sourceDir).WithLogger(layer.GetLogger()).DryRun()
		if err == nil {
			return output, nil
		}
		a.logDebug("retrying apply", layer)
	}
	return output, errors.WithMessagef(err, "%s - failed to run dry run apply", logging.CallerStr(logging.Me))
}

func (a KubectlLayerApplier) getSourceResources(layer layers.Layer) (objs []runtime.Object, err error) {
	logging.TraceCall(a.getLog(layer))
	defer logging.TraceExit(a.getLog(layer))
	sourceDir, err := a.checkSourcePath(layer)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to check source path")
	}

	output, err := a.doApply(layer, sourceDir)
	if err != nil {
		return nil, errors.WithMessagef(err, "%s - failed to execute kubectl while parsing source directory (%s) for AddonsLayer %s",
			logging.CallerStr(logging.Me), sourceDir, layer.GetName())
	}

	objs, err = a.decodeAddons(layer, output)
	if err != nil {
		return nil, errors.WithMessagef(err, "%s - failed to decode apply dry run output", logging.CallerStr(logging.Me))
	}

	err = a.addOwnerRefs(layer, objs)
	if err != nil {
		return nil, errors.WithMessagef(err, "%s - failed to add owner reference", logging.CallerStr(logging.Me))
	}

	return objs, nil
}

func (a KubectlLayerApplier) getSourceHelmReleases(layer layers.Layer) (hrs map[string]*helmctlv2.HelmRelease, err error) {
	logging.TraceCall(a.getLog(layer))
	defer logging.TraceExit(a.getLog(layer))
	objs, err := a.getSourceResources(layer)
	if err != nil {
		return nil, errors.WithMessagef(err, "%s - failed to get source helm releases", logging.CallerStr(logging.Me))
	}

	sourceHrs, err := a.decodeHelmReleases(layer, objs)
	if err != nil {
		return nil, errors.WithMessagef(err, "%s - failed to decode helm releases", logging.CallerStr(logging.Me))
	}

	// Check for kraan.updateVersion annotation
	for _, source := range sourceHrs {
		if err := a.processUpdateVersionAnnotation(layer, source); err != nil {
			return nil, errors.WithMessagef(err, "%s - failed to process updateVersion annotation", logging.CallerStr(logging.Me))
		}
	}

	hrs = map[string]*helmctlv2.HelmRelease{}
	for _, hr := range sourceHrs {
		hrs[getLabel(hr.ObjectMeta)] = hr
	}

	return hrs, nil
}

func (a KubectlLayerApplier) getSourceHelmRepos(layer layers.Layer) (hrs []*sourcev1.HelmRepository, err error) {
	logging.TraceCall(a.getLog(layer))
	defer logging.TraceExit(a.getLog(layer))
	objs, err := a.getSourceResources(layer)
	if err != nil {
		return nil, errors.WithMessagef(err, "%s - failed to get source helm repositories", logging.CallerStr(logging.Me))
	}

	hrs, err = a.decodeHelmRepos(layer, objs)
	if err != nil {
		return nil, errors.WithMessagef(err, "%s - failed to decode helm repositories", logging.CallerStr(logging.Me))
	}

	return hrs, nil
}

// GetSourceAndClusterHelmReleases returns source and cluster helmreleases
func (a KubectlLayerApplier) GetSourceAndClusterHelmReleases(ctx context.Context, layer layers.Layer) (sourceHrs, clusterHrs map[string]*helmctlv2.HelmRelease, err error) {
	logging.TraceCall(a.getLog(layer))
	defer logging.TraceExit(a.getLog(layer))

	sourceHrs, err = a.getSourceHelmReleases(layer)
	if err != nil {
		return nil, nil, errors.WithMessagef(err, "%s - failed to get source helm releases", logging.CallerStr(logging.Me))
	}

	clusterHrs, err = a.GetHelmReleases(ctx, layer)
	if err != nil {
		return nil, nil, errors.WithMessagef(err, "%s - failed to get helm releases", logging.CallerStr(logging.Me))
	}
	return sourceHrs, clusterHrs, nil
}

// Apply an AddonLayer to the cluster.
func (a KubectlLayerApplier) Apply(ctx context.Context, layer layers.Layer) (err error) {
	logging.TraceCall(a.getLog(layer))
	defer logging.TraceExit(a.getLog(layer))
	a.logDebug("applying", layer)

	sourceHrs, err := a.getSourceHelmReleases(layer)
	if err != nil {
		return errors.WithMessagef(err, "%s - failed to get source helm releases", logging.CallerStr(logging.Me))
	}

	err = a.applyHelmReleaseObjects(ctx, layer, sourceHrs)
	if err != nil {
		return errors.WithMessagef(err, "%s - failed to apply helmrelease objects", logging.CallerStr(logging.Me))
	}

	hrs, err := a.getSourceHelmRepos(layer)
	if err != nil {
		return errors.WithMessagef(err, "%s - failed to get source helm repos", logging.CallerStr(logging.Me))
	}

	err = a.applyHelmRepoObjects(ctx, layer, hrs)
	if err != nil {
		return errors.WithMessagef(err, "%s - failed to apply helmrepo objects", logging.CallerStr(logging.Me))
	}
	return nil
}

func layerOwner(obj metav1.Object) string {
	for _, owner := range obj.GetOwnerReferences() {
		if owner.Kind == "AddonsLayer" && owner.APIVersion == "kraan.io/v1alpha1" {
			return owner.Name
		}
	}
	return ""
}

func changeOwner(layer layers.Layer, hr *helmctlv2.HelmRelease) {
	for index, owner := range hr.OwnerReferences {
		if owner.Kind == "AddonsLayer" && owner.APIVersion == "kraan.io/v1alpha1" && owner.Name != layer.GetName() {
			hr.OwnerReferences[index].Name = layer.GetName()
			hr.OwnerReferences[index].UID = layer.GetAddonsLayer().UID
		}
	}
}

func getTimestamp(dtg string) (*metav1.Time, error) {
	t, err := time.Parse(time.RFC3339, dtg)
	if err != nil {
		return nil, errors.Wrapf(err, "%s - failed to parse timestamp '%s'", logging.CallerStr(logging.Me), dtg)
	}

	mt := metav1.NewTime(t)

	return &mt, nil
}

func labelValue(labelName string, obj *metav1.Object) string {
	labels := (*obj).GetLabels()

	if labels == nil {
		return ""
	}

	if value, ok := labels[labelName]; ok {
		return value
	}
	return ""
}

func (a KubectlLayerApplier) orphanLabel(ctx context.Context, hr *helmctlv2.HelmRelease) (*metav1.Time, error) {
	labels := hr.GetLabels()
	for label, value := range labels {
		if label == orphanedLabel {
			dtg, err := getTimestamp(strings.ReplaceAll(value, ".", ":"))
			if err != nil {
				return nil, errors.WithMessagef(err, "%s - failed to parse orphaned label value as timestamp", logging.CallerStr(logging.Me))
			}
			return dtg, nil
		}
	}
	// Label not present, add it
	now := metav1.Now()
	labels[orphanedLabel] = strings.ReplaceAll(now.Format(time.RFC3339), ":", ".")
	hr.SetLabels(labels)
	err := a.client.Update(ctx, hr, &client.UpdateOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "%s - failed to Update helmRelease '%s'", logging.CallerStr(logging.Me), getObjLabel(hr))
	}
	return &now, nil
}

// Adopt sets an orphaned HelmRelease to a new owner.
// It returns a bool value set to true if the HelmRelease is still waiting for adoption or false if the adoption period has expired.
func (a KubectlLayerApplier) Adopt(ctx context.Context, layer layers.Layer, hr *helmctlv2.HelmRelease) error {
	logging.TraceCall(a.getLog(layer))
	defer logging.TraceExit(a.getLog(layer))

	labels := hr.GetLabels()
	delete(labels, orphanedLabel)
	if labels == nil {
		labels = map[string]string{}
	}
	labels[ownerLabel] = layer.GetName()
	hr.SetLabels(labels)

	changeOwner(layer, hr)

	err := a.client.Update(ctx, hr, &client.UpdateOptions{})
	if err != nil {
		return errors.Wrapf(err, "%s - failed to Update helmRelease '%s'", logging.CallerStr(logging.Me), getObjLabel(hr))
	}

	return nil
}

// Orphan adds an 'orphan' label to a HelmRelease if not already present.
// It returns a bool value set to true if the HelmRelease is still waiting for adoption or false if the adoption period has expired.
func (a KubectlLayerApplier) Orphan(ctx context.Context, layer layers.Layer, hr *helmctlv2.HelmRelease) (pending bool, err error) {
	logging.TraceCall(a.getLog(layer))
	defer logging.TraceExit(a.getLog(layer))

	// label HR to as orphan if not already labelled, set label value to time now.
	key, err := client.ObjectKeyFromObject(hr)
	if err != nil {
		return false, errors.Wrapf(err, "%s - failed to get an ObjectKey '%s'", logging.CallerStr(logging.Me), getObjLabel(hr))
	}
	existing := hr.DeepCopyObject()
	err = a.client.Get(ctx, key, existing)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			removeResourceVersion(hr)
			a.logDebug("HelmRelease not found", layer, logging.GetObjKindNamespaceName(hr)...)
			return false, nil
		}
		return false, errors.Wrapf(err, "%s - failed to get HelmRelease '%s'", logging.CallerStr(logging.Me), key)
	}
	theHr, ok := existing.(*helmctlv2.HelmRelease)
	if !ok {
		return false, fmt.Errorf("failed to convert runtime.Object to HelmRelease")
	}

	if layer.GetName() != layerOwner(theHr) {
		a.logDebug("Layer no longer owns HelmRelease", layer, logging.GetObjKindNamespaceName(hr)...)
		return false, nil
	}

	orphanedTime, err := a.orphanLabel(ctx, theHr)
	if err != nil {
		return false, errors.WithMessagef(err, "%s - failed to get/set orphan label '%s'", logging.CallerStr(logging.Me), getObjLabel(hr))
	}

	return orphanedTime.Add(layer.GetDelay()).After(time.Now()), nil
}

// Prune the AddonsLayer by removing the Addons found in the cluster that have since been removed from the Layer.
func (a KubectlLayerApplier) Prune(ctx context.Context, layer layers.Layer, pruneHrs []*helmctlv2.HelmRelease) (err error) {
	logging.TraceCall(a.getLog(layer))
	defer logging.TraceExit(a.getLog(layer))
	for _, hr := range pruneHrs {
		err := a.client.Delete(ctx, hr, client.PropagationPolicy(metav1.DeletePropagationBackground))
		if err != nil {
			return errors.Wrapf(err, "%s - unable to delete HelmRelease '%s' for AddonsLayer '%s'",
				logging.CallerStr(logging.Me), getLabel(hr.ObjectMeta), layer.GetName())
		}
	}
	return nil
}

// getResourceInfo updates a resource object with details from object on cluster
func (a KubectlLayerApplier) getResourceInfo(layer layers.Layer, resource kraanv1alpha1.Resource, conditions []metav1.Condition) kraanv1alpha1.Resource {
	logging.TraceCall(a.getLog(layer))
	defer logging.TraceExit(a.getLog(layer))

	cond := a.getReadyCondition(layer, conditions)
	if cond == nil {
		resource.Status = kraanv1alpha1.NotDeployed
		return resource
	}
	resource.LastTransitionTime = cond.LastTransitionTime

	if cond.Status == metav1.ConditionTrue {
		resource.Status = kraanv1alpha1.Deployed
		return resource
	}
	resource.Status = cond.Reason
	return resource
}

func (a KubectlLayerApplier) getReadyCondition(layer layers.Layer, conditions []metav1.Condition) *metav1.Condition {
	logging.TraceCall(a.getLog(layer))
	defer logging.TraceExit(a.getLog(layer))

	for _, cond := range conditions {
		if cond.Type == "Ready" {
			return &cond
		}
	}
	return nil
}

// GetResources returns a list of resources for this AddonsLayer
func (a KubectlLayerApplier) GetResources(ctx context.Context, layer layers.Layer) (resources []kraanv1alpha1.Resource, err error) {
	logging.TraceCall(a.getLog(layer))
	defer logging.TraceExit(a.getLog(layer))

	sourceHrs, clusterHrs, err := a.GetSourceAndClusterHelmReleases(ctx, layer)
	if err != nil {
		return nil, errors.WithMessagef(err, "%s - failed to get helm releases", logging.CallerStr(logging.Me))
	}

	for key, source := range sourceHrs {
		resource := kraanv1alpha1.Resource{
			Namespace:          source.GetNamespace(),
			Name:               source.GetName(),
			Kind:               "helmreleases.helm.toolkit.fluxcd.io",
			LastTransitionTime: metav1.Now(),
			Status:             "Unknown",
		}
		hr, ok := clusterHrs[key]
		if ok {
			a.logDebug("HelmRelease in AddonsLayer source directory and on cluster", layer, logging.GetObjKindNamespaceName(source)...)
			resources = append(resources, a.getResourceInfo(layer, resource, hr.Status.Conditions))
		} else {
			// this resource exists in the source directory but not on the cluster
			a.logDebug("HelmRelease in AddonsLayer source directory but not on cluster", layer, logging.GetObjKindNamespaceName(source)...)
			resource.Status = kraanv1alpha1.NotDeployed
			resources = append(resources, resource)
		}
	}

	for key, hr := range clusterHrs {
		resource := kraanv1alpha1.Resource{
			Namespace:          hr.GetNamespace(),
			Name:               hr.GetName(),
			Kind:               "helmreleases.helm.toolkit.fluxcd.io",
			LastTransitionTime: metav1.Now(),
			Status:             "Unknown",
		}
		_, ok := sourceHrs[key]
		if !ok {
			a.logDebug("HelmRelease not in AddonsLayer source directory but on cluster", layer, "name", clusterHrs[key])
			resources = append(resources, a.getResourceInfo(layer, resource, hr.Status.Conditions))
		}
	}
	return resources, err
}

// PruneIsRequired returns true if any resources need to be pruned for this AddonsLayer
func (a KubectlLayerApplier) PruneIsRequired(ctx context.Context, layer layers.Layer) (pruneRequired bool, pruneHrs []*helmctlv2.HelmRelease, err error) {
	logging.TraceCall(a.getLog(layer))
	defer logging.TraceExit(a.getLog(layer))

	sourceHrs, clusterHrs, err := a.GetSourceAndClusterHelmReleases(ctx, layer)
	if err != nil {
		return false, nil, errors.WithMessagef(err, "%s - failed to get helm releases", logging.CallerStr(logging.Me))
	}

	pruneRequired = false
	pruneHrs = []*helmctlv2.HelmRelease{}

	for key, hr := range clusterHrs {
		_, ok := sourceHrs[key]
		if !ok {
			// this resource exists on the cluster but not in the source directory
			a.logDebug("pruned HelmRelease for AddonsLayer in KubeAPI but not in source directory", layer, logging.GetObjKindNamespaceName(hr)...)
			pruneRequired = true
			pruneHrs = append(pruneHrs, hr)
		}
	}

	return pruneRequired, pruneHrs, nil
}

// processUpdateVersionAnnotation checks for kraan.updateVersion annotation set to true and updates values to include current version.
func (a KubectlLayerApplier) processUpdateVersionAnnotation(layer layers.Layer, source *helmctlv2.HelmRelease) error {
	annotations := source.GetAnnotations()
	for k, v := range annotations {
		if k == "kraan.updateVersion" && v == "true" {
			a.logTrace("values for chart with annotation", layer, append(logging.GetObjKindNamespaceName(source), "values", logging.LogJSON(source.Spec.Values))...)
			values := make(map[string]interface{})
			if source.Spec.Values != nil {
				err := json.Unmarshal(source.Spec.Values.Raw, &values)
				if err != nil {
					return errors.WithMessagef(err, "%s - failed convert values to json string", logging.CallerStr(logging.Me))
				}
			}
			values["kraanVersion"] = layer.GetSpec().Version
			newJSON, err := json.Marshal(values)
			if err != nil {
				return errors.WithMessagef(err, "%s - failed convert updated values to json string", logging.CallerStr(logging.Me))
			}
			source.Spec.Values = &apiextensionsv1.JSON{Raw: newJSON}
			a.logTrace("values for chart after update", layer, append(logging.GetObjKindNamespaceName(source), "values", logging.LogJSON(source.Spec.Values))...)
		}
	}
	return nil
}

// ApplyIsRequired returns true if any resources need to be applied for this AddonsLayer
func (a KubectlLayerApplier) ApplyIsRequired(ctx context.Context, layer layers.Layer) (applyIsRequired bool, err error) {
	logging.TraceCall(a.getLog(layer))
	defer logging.TraceExit(a.getLog(layer))

	sourceHrs, clusterHrs, err := a.GetSourceAndClusterHelmReleases(ctx, layer)
	if err != nil {
		return false, errors.WithMessagef(err, "%s - failed to get helm releases", logging.CallerStr(logging.Me))
	}

	// Check for any missing resources first.  This is the fastest and easiest check.
	for key, source := range sourceHrs {
		_, ok := clusterHrs[key]
		if !ok {
			// this resource exists in the source directory but not on the cluster
			a.logDebug("found new HelmRelease in AddonsLayer source directory", layer, logging.GetObjKindNamespaceName(source)...)
			return true, nil
		}
	}

	// Compare each HelmRelease source spec to the spec of the found HelmRelease on the cluster
	for key, source := range sourceHrs {
		if a.sourceHasReleaseChanged(layer, source, clusterHrs[key]) {
			a.logDebug("found source change", layer, logging.GetObjKindNamespaceName(source)...)
			return true, nil
		}
	}
	return a.helmReposApplyRequired(ctx, layer)
}

func (a KubectlLayerApplier) helmReposApplyRequired(ctx context.Context, layer layers.Layer) (applyIsRequired bool, err error) {
	logging.TraceCall(a.getLog(layer))
	defer logging.TraceExit(a.getLog(layer))
	sourceHrs, err := a.getSourceHelmRepos(layer)
	if err != nil {
		return false, errors.WithMessagef(err, "%s - failed to get source helm repositories", logging.CallerStr(logging.Me))
	}

	clusterHrs, err := a.getHelmRepos(ctx, layer)
	if err != nil {
		return false, errors.WithMessagef(err, "%s - failed to get helm repositories", logging.CallerStr(logging.Me))
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
			a.logDebug("found new HelmRepository in AddonsLayer source directory", layer, logging.GetObjKindNamespaceName(source)...)
			return true, nil
		}
	}

	// Compare each HelmReposity source spec to the spec of the found HelmReposity on the cluster
	for _, source := range sourceHrs {
		found := hrs[getLabel(source.ObjectMeta)]
		if a.sourceHasRepoChanged(layer, source, found) {
			a.logDebug("found source change", layer, logging.GetObjKindNamespaceName(source)...)
			return true, nil
		}
	}
	return false, nil
}

func (a KubectlLayerApplier) sourceHasReleaseChanged(layer layers.Layer, source, found *helmctlv2.HelmRelease) (changed bool) {
	logging.TraceCall(a.getLog(layer))
	defer logging.TraceExit(a.getLog(layer))
	if !CompareAsJSON(source.Spec, found.Spec) {
		a.logDebug("found spec change for HelmRelease in AddonsLayer source directory", layer,
			append(logging.GetObjKindNamespaceName(source), "source", source.Spec, "found", found.Spec, "diff", cmp.Diff(source.Spec, found.Spec))...)
		return true
	}
	if !reflect.DeepEqual(source.ObjectMeta.Labels, found.ObjectMeta.Labels) {
		// this resource source spec does not match the resource spec on the cluster
		a.logDebug("found label change for HelmRelease in AddonsLayer source directory", layer,
			append(logging.GetObjKindNamespaceName(source), "label source", source.ObjectMeta.Labels, "label found", found.ObjectMeta.Labels)...)
		return true
	}
	a.logTrace("found no changes for HelmRelease in AddonsLayer source directory", layer)
	return false
}

func (a KubectlLayerApplier) sourceHasRepoChanged(layer layers.Layer, source, found *sourcev1.HelmRepository) (changed bool) {
	logging.TraceCall(a.getLog(layer))
	defer logging.TraceExit(a.getLog(layer))
	if !CompareAsJSON(source.Spec, found.Spec) {
		a.logDebug("found spec change for HelmRepository in AddonsLayer source directory", layer,
			append(logging.GetObjKindNamespaceName(source), "source", source.Spec, "found", found.Spec, "diff", cmp.Diff(source.Spec, found.Spec))...)
		return true
	}
	if !reflect.DeepEqual(source.ObjectMeta.Labels, found.ObjectMeta.Labels) {
		// this resource source spec does not match the resource spec on the cluster
		a.logDebug("found label change for HelmRepository in AddonsLayer source directory", layer,
			append(logging.GetObjKindNamespaceName(source), "label source", source.ObjectMeta.Labels, "label found", found.ObjectMeta.Labels)...)
		return true
	}
	a.logDebug("found no change for HelmRepositories in AddonsLayer source directory", layer)
	return false
}

// ApplyWasSuccessful returns true if all of the resources in this AddonsLayer are in the Success phase
func (a KubectlLayerApplier) ApplyWasSuccessful(ctx context.Context, layer layers.Layer) (applyIsRequired bool, hrName string, err error) {
	logging.TraceCall(a.getLog(layer))
	defer logging.TraceExit(a.getLog(layer))
	clusterHrs, err := a.GetHelmReleases(ctx, layer)
	if err != nil {
		return false, "", errors.WithMessagef(err, "%s - failed to get helm releases", logging.CallerStr(logging.Me))
	}

	for key, hr := range clusterHrs {
		if !fluxmeta.InReadyCondition(hr.Status.Conditions) {
			a.logInfo("unsuccessful HelmRelease deployment", layer, append(logging.GetObjKindNamespaceName(hr), "resource", hr)...)
			return false, key, nil
		}
	}

	return true, "", nil
}

func CompareAsJSON(one, two interface{}) bool {
	if one == nil && two == nil {
		return true
	}
	jsonOne, err := toJSON(one)
	if err != nil {
		return false
	}

	jsonTwo, err := toJSON(two)
	if err != nil {
		return false
	}
	return jsonOne == jsonTwo
}

// toJSON is used to convert a data structure into JSON format.
func toJSON(data interface{}) (string, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", errors.WithMessagef(err, "%s - failed to marshal json", logging.CallerStr(logging.Me))
	}
	var prettyJSON bytes.Buffer
	err = json.Indent(&prettyJSON, jsonData, "", "\t")
	if err != nil {
		return "", errors.WithMessagef(err, "%s - failed indent json", logging.CallerStr(logging.Me))
	}
	return prettyJSON.String(), nil
}
