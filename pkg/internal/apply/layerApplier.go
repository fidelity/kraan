//Package apply xxx
// TEMP DISABLED - go:generate mockgen -destination=mockLayerApplier.go -package=apply -source=layerApplier.go . LayerApplier
// re-enable the go:generate annotation when we're ready to write tests for the controller
package apply

/*
To generate mock code for the LayerApplier run 'go generate ./...' from the project root directory.
*/
import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/fidelity/kraan/pkg/internal/kubectl"
	"github.com/fidelity/kraan/pkg/internal/layers"

	helmopv1 "github.com/fluxcd/helm-operator/pkg/apis/helm.fluxcd.io/v1"
	helmrelease "github.com/fluxcd/helm-operator/pkg/client/clientset/versioned"
	helmopscheme "github.com/fluxcd/helm-operator/pkg/client/clientset/versioned/scheme"
	hrclientv1 "github.com/fluxcd/helm-operator/pkg/client/clientset/versioned/typed/helm.fluxcd.io/v1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	kscheme "k8s.io/client-go/kubernetes/scheme"
)

var (
	newKubectlFunc func(logger logr.Logger) (kubectl.Kubectl, error) = kubectl.NewKubectl
)

func init() {
	helmopscheme.AddToScheme(kscheme.Scheme) // nolint:errcheck // ok
}

// LayerApplier defines methods for managing the Addons within an AddonLayer in a cluster.
type LayerApplier interface {
	Apply(layer layers.Layer) (err error)
	Prune(layer layers.Layer) (err error)
}

// KubectlLayerApplier applies an AddonsLayer to a Kubernetes cluster using the kubectl command.
type KubectlLayerApplier struct {
	kubectl kubectl.Kubectl
	logger  logr.Logger
}

// NewApplier returns a LayerApplier instance.
func NewApplier(logger logr.Logger) (applier LayerApplier, err error) {
	kubectl, err := newKubectlFunc(logger)
	if err != nil {
		return nil, err
	}
	applier = &KubectlLayerApplier{
		kubectl: kubectl,
		logger:  logger,
	}
	return applier, nil
}

func (a *KubectlLayerApplier) getLog(layer layers.Layer) (logger logr.Logger) {
	logger = layer.GetLogger()
	if logger == nil {
		logger = a.logger
	}
	return logger
}

func (a *KubectlLayerApplier) log(level int, msg string, layer layers.Layer, keysAndValues ...interface{}) {
	a.getLog(layer).V(level).Info(msg, append(keysAndValues, "sourcePath", layer.GetSourcePath(), "layer", layer)...)
}

func (a *KubectlLayerApplier) logError(err error, msg string, layer layers.Layer, keysAndValues ...interface{}) {
	a.getLog(layer).Error(err, msg, append(keysAndValues, "sourcePath", layer.GetSourcePath(), "layer", layer)...)
}

func (a *KubectlLayerApplier) logInfo(msg string, layer layers.Layer, keysAndValues ...interface{}) {
	a.log(1, msg, layer, keysAndValues...)
}

func (a *KubectlLayerApplier) logDebug(msg string, layer layers.Layer, keysAndValues ...interface{}) {
	a.log(5, msg, layer, keysAndValues...)
}

func (a *KubectlLayerApplier) logTrace(msg string, layer layers.Layer, keysAndValues ...interface{}) {
	a.log(7, msg, layer, keysAndValues...)
}

func (a *KubectlLayerApplier) logErrors(errz []error, layer layers.Layer) {
	for _, err := range errz {
		a.logError(err, "error while applying layer", layer)
	}
}

func (a *KubectlLayerApplier) getAddons(hrClient *helmrelease.Clientset, hrs []*helmopv1.HelmRelease, layer layers.Layer) (foundHrs []*helmopv1.HelmRelease) {
	for i, hr := range hrs {
		a.logDebug("Checking HelmRelease for AddonsLayer", layer, "index", i, "helmRelease", hr)
		nsClient := hrClient.HelmV1().HelmReleases(hr.Namespace)
		foundHr, err := nsClient.Get(hr.Name, metav1.GetOptions{TypeMeta: hr.TypeMeta})
		if err == nil {
			a.logDebug("Found HelmRelease for AddonsLayer", layer, "index", i, "helmRelease", foundHr)
			foundHrs = append(foundHrs, foundHr)
		} else {
			err := fmt.Errorf("Error getting HelmRelease '%s' for AddonsLayer '%s'", getLabel(hr), layer.GetName())
			a.logError(err, "Error getting HelmRelease for AddonsLayer", layer, "index", i, "helmRelease", hr)
		}
	}
	return foundHrs
}

func getLabel(hr *helmopv1.HelmRelease) string {
	return fmt.Sprintf("%s/%s", hr.GetNamespace(), hr.GetName())
}

type addonError struct {
	label   string
	err     error
	isFatal bool
}

func newEventError(event *watch.Event, hr *helmopv1.HelmRelease) addonError {
	err := fmt.Errorf("Received %s event for HelmRelease %s", event.Type, getLabel(hr))
	return addonError{
		label:   getLabel(hr),
		err:     err,
		isFatal: event.Type == watch.Deleted,
	}
}

func newError(msg string, hr *helmopv1.HelmRelease) addonError {
	err := fmt.Errorf("%s for HelmRelease %s", msg, getLabel(hr))
	return addonError{
		label:   getLabel(hr),
		err:     err,
		isFatal: false,
	}
}

func newFatalError(err error, hr *helmopv1.HelmRelease) addonError {
	return addonError{
		label:   getLabel(hr),
		err:     err,
		isFatal: true,
	}
}

func (p *addonError) getLabel() string {
	return p.label
}

type addonPhase struct {
	label string
	phase *helmopv1.HelmReleasePhase
	state string
}

func newAddonPhase(hr *helmopv1.HelmRelease) addonPhase {
	return addonPhase{
		label: getLabel(hr),
		phase: &hr.Status.Phase,
		state: hr.Status.ReleaseStatus,
	}
}

func (p *addonPhase) getLabel() string {
	return p.label
}

type addonsTracker struct {
	addons   []*helmopv1.HelmRelease
	phases   map[string]*helmopv1.HelmReleasePhase
	states   map[string]string
	errors   map[string][]*error
	success  map[string]bool
	timedout bool
	mutex    *sync.Mutex
}

func newTracker(addons []*helmopv1.HelmRelease) *addonsTracker {
	tracker := &addonsTracker{
		addons:   addons,
		phases:   make(map[string]*helmopv1.HelmReleasePhase, len(addons)),
		states:   make(map[string]string, len(addons)),
		errors:   make(map[string][]*error, len(addons)),
		success:  make(map[string]bool, len(addons)),
		timedout: false,
		mutex:    &sync.Mutex{},
	}
	for _, addon := range addons {
		tracker.initPhase(addon)
	}
	return tracker
}

func (p *addonsTracker) getPhase(hr *helmopv1.HelmRelease) (phase *helmopv1.HelmReleasePhase, found bool) {
	p.mutex.Lock()
	phase, found = p.phases[getLabel(hr)]
	p.mutex.Unlock()
	return phase, found
}

func (p *addonsTracker) getState(hr *helmopv1.HelmRelease) (state string, found bool) {
	p.mutex.Lock()
	state, found = p.states[getLabel(hr)]
	p.mutex.Unlock()
	return state, found
}

func (p *addonsTracker) getErrors(hr *helmopv1.HelmRelease) (errors []*error, found bool) {
	p.mutex.Lock()
	errors, found = p.errors[getLabel(hr)]
	p.mutex.Unlock()
	return errors, found
}

func (p *addonsTracker) initPhase(hr *helmopv1.HelmRelease) {
	if !isObserved(hr) {
		return
	}
	phase := &hr.Status.Phase
	state := hr.Status.ReleaseStatus
	label := getLabel(hr)
	p.phases[label] = phase
	p.states[label] = state
	if isSuccessPhase(phase) {
		p.success[label] = true
	} else if isFailedPhase(phase) {
		p.success[label] = false
	}
}

func (p *addonsTracker) updatePhase(phase addonPhase) {
	p.mutex.Lock()
	p.phases[phase.label] = phase.phase
	p.states[phase.label] = phase.state
	if isSuccessPhase(phase.phase) {
		p.success[phase.label] = true
	} else if isFailedPhase(phase.phase) {
		p.success[phase.label] = false
	}
	p.mutex.Unlock()
}

func (p *addonsTracker) addError(err addonError) {
	p.mutex.Lock()
	p.errors[err.label] = append(p.errors[err.label], &err.err)
	if err.isFatal {
		p.success[err.label] = false
	}
	p.mutex.Unlock()
}

func (p *addonsTracker) timeout() {
	p.mutex.Lock()
	p.timedout = true
	for _, hr := range p.addons {
		label := getLabel(hr)
		// Add an error for this addon if it has not yet succeeded or failed
		_, found := p.success[label]
		if !found {
			phase := p.phases[label]
			state := p.states[label]
			err := fmt.Errorf("HelmRelease '%s' timed out in phase '%s' releaseStatus '%s'", label, *phase, state)
			p.errors[label] = append(p.errors[label], &err)
		}
	}
	p.mutex.Unlock()
}

func (p *addonsTracker) isDone(hr *helmopv1.HelmRelease) bool {
	p.mutex.Lock()
	_, found := p.success[getLabel(hr)]
	p.mutex.Unlock()
	return found
}

func (p *addonsTracker) isSuccess(hr *helmopv1.HelmRelease) bool {
	p.mutex.Lock()
	success, found := p.success[getLabel(hr)]
	if !found {
		success = false
	}
	p.mutex.Unlock()
	return success
}

func (p *addonsTracker) releaseDone() bool {
	for _, hr := range p.addons {
		if !p.isDone(hr) {
			return false
		}
	}
	return true
}

func (p *addonsTracker) releaseFailed() bool {
	for _, hr := range p.addons {
		if !p.isSuccess(hr) {
			return true
		}
	}
	return false
}

func (p *addonsTracker) releaseTimedout() bool {
	return p.timedout
}

func (p *addonsTracker) logReleaseErrors(a *KubectlLayerApplier, layer layers.Layer) {
	for _, hr := range p.addons {
		if p.isSuccess(hr) {
			continue
		}
		errors, found := p.errors[getLabel(hr)]
		if found {
			for _, err := range errors {
				a.logError(*err, "HelmRelease failed for AddonsLayer", layer, "helmRelease", hr)
			}
		} else {
			err := fmt.Errorf("no errors tracked for HelmRelease '%s' - it may have failed before tracking started", getLabel(hr))
			a.logError(err, "HelmRelease failed for AddonsLayer", layer, "helmRelease", hr)
		}
	}
}

func isObserved(hr *helmopv1.HelmRelease) bool {
	return hr.GetGeneration() == hr.Status.ObservedGeneration
}

func isSuccessPhase(phase *helmopv1.HelmReleasePhase) bool {
	switch *phase {
	case helmopv1.HelmReleasePhaseSucceeded:
		return true
	default:
		return false
	}
}

func isErrorPhase(phase *helmopv1.HelmReleasePhase) bool {
	switch *phase {
	case helmopv1.HelmReleasePhaseChartFetchFailed,
		helmopv1.HelmReleasePhaseDeployFailed,
		helmopv1.HelmReleasePhaseRollbackFailed,
		helmopv1.HelmReleasePhaseTestFailed:
		return true
	default:
		return false
	}
}

func isFailedPhase(phase *helmopv1.HelmReleasePhase) bool {
	switch *phase {
	case helmopv1.HelmReleasePhaseFailed:
		return true
	default:
		return false
	}
}

func (a *KubectlLayerApplier) groupHrsByNamespace(hrs []*helmopv1.HelmRelease) map[string][]*helmopv1.HelmRelease {
	// Group HelmRelease items by Namespace as the client is Namespace-scoped
	hrsByNamespace := make(map[string][]*helmopv1.HelmRelease)
	for _, hr := range hrs {
		hrsByNamespace[hr.Namespace] = append(hrsByNamespace[hr.Namespace], hr)
	}
	return hrsByNamespace
}

type addonsWatcher struct {
	applier        *KubectlLayerApplier
	clientset      *helmrelease.Clientset
	hrs            []*helmopv1.HelmRelease
	tracker        *addonsTracker
	phases         chan addonPhase
	errors         chan addonError
	layer          layers.Layer
	watcherTimeout int64
	timer          *time.Timer
}

func (a *KubectlLayerApplier) newWatcher(layer layers.Layer, hrClientSet *helmrelease.Clientset, hrs []*helmopv1.HelmRelease, timeoutDuration time.Duration) *addonsWatcher {
	return &addonsWatcher{
		applier:        a,
		clientset:      hrClientSet,
		hrs:            a.getAddons(hrClientSet, hrs, layer),
		tracker:        newTracker(hrs),
		phases:         make(chan addonPhase, len(hrs)),
		errors:         make(chan addonError, len(hrs)),
		layer:          layer,
		watcherTimeout: int64(timeoutDuration) + 30,
		timer:          time.NewTimer(timeoutDuration),
	}
}

func (w *addonsWatcher) getWatcherTimeout() *int64 {
	return &w.watcherTimeout
}

func (w *addonsWatcher) watchHelmRelease(hr *helmopv1.HelmRelease, client hrclientv1.HelmReleaseInterface, phases chan<- addonPhase, errors chan<- addonError) {
	a := w.applier
	listOptions := metav1.SingleObject(hr.ObjectMeta)
	listOptions.TimeoutSeconds = w.getWatcherTimeout()
	/*listOptions := metav1.ListOptions{
		FieldSelector:  fmt.Sprintf("meta.name=%s", watchedHr.GetName()),
		TimeoutSeconds: &watcherTimeout,
	}*/

	// TODO - investigate using a CustomPredicate here to filter for only the events we're interested in.
	watcher, err := client.Watch(listOptions)
	if err != nil {
		wrapErr := fmt.Errorf("error creating Watch for HelmRelease '%s' in AddonsLayer '%s': %w", getLabel(hr), layerLabel(w.layer), err)
		errors <- newFatalError(wrapErr, hr)
		return
	}
	// TODO: Do I need to explicitly call Stop on the Watcher?
	// defer watcher.Stop()
	eventChannel := watcher.ResultChan()
	// TODO: will the range exit when the watcherTimeout ends?
	for event := range eventChannel {
		eventHr, ok := event.Object.(*helmopv1.HelmRelease)
		if !ok {
			// Somehow we got an event for an unexpected type!
			msg := fmt.Sprintf("Watcher returned event '%s' with an unexpected object type (%T)", event.Type, event.Object)
			errors <- newError(msg, eventHr)
		}

		switch event.Type {
		case watch.Added, watch.Modified:
			a.logDebug("Recieved event for HelmRelease", w.layer, "helmRelease", eventHr, "event", event)
			if isObserved(eventHr) {
				phases <- newAddonPhase(eventHr)
			}
		case watch.Deleted, watch.Error:
			errors <- newEventError(&event, eventHr)
		}
		phase := &eventHr.Status.Phase
		if isSuccessPhase(phase) || isFailedPhase(phase) {
			return // TODO - verify that this stops the Watcher and closes its channel.
		}
	}
}

func (w *addonsWatcher) catchEvents(phaseChannel <-chan addonPhase, errorChannel <-chan addonError) *addonsWatcher {
	a := w.applier
	tracker := w.tracker
	layer := w.layer
	timeoutChannel := w.timer.C
catchLoop:
	for {
		select {
		case phase, ok := <-phaseChannel:
			if !ok {
				a.logInfo("AddonsLayer phase channel closed", layer)
				break catchLoop
			}
			tracker.updatePhase(phase)
			if tracker.releaseDone() {
				a.logInfo("AddonsLayer release completed!", layer)
				break catchLoop
			}
		case addonError, ok := <-errorChannel:
			if !ok {
				a.logInfo("AddonsLayer error channel closed", layer)
				break catchLoop
			}
			tracker.addError(addonError)
		case timeout := <-timeoutChannel:
			err := fmt.Errorf("Watch Timeout! %#v", timeout)
			a.logError(err, "AddonsLayer watch timeout", layer, "timeout", timeout)
			tracker.timeout()
			break catchLoop
		}
	}
	return w
}

func (w *addonsWatcher) makeNamespaceWatchers(ns string, hrs []*helmopv1.HelmRelease) {
	client := w.clientset.HelmV1().HelmReleases(ns)
	for _, hr := range hrs {
		// Skip creating the watcher if this HelmRelease is already in a terminal state
		if w.tracker.isDone(hr) {
			continue
		}
		go w.watchHelmRelease(hr, client, w.phases, w.errors)
	}
}

func (w *addonsWatcher) returnResults() *addonsTracker {
	return w.tracker
}

func (w *addonsWatcher) startWatchers() *addonsWatcher {
	hrsByNamespace := w.applier.groupHrsByNamespace(w.hrs)
	for ns, hrs := range hrsByNamespace {
		w.makeNamespaceWatchers(ns, hrs)
	}
	return w
}

func (w *addonsWatcher) watchHrs() *addonsTracker {
	return w.startWatchers().catchEvents(w.phases, w.errors).returnResults()
}

func (a *KubectlLayerApplier) decodeAddons(layer layers.Layer, json []byte) (hrs []*helmopv1.HelmRelease, errz []error, err error) {
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
		msg := fmt.Sprintf("decoded kubectl output was not a HelmRelease or List: %s", string(json))
		err = fmt.Errorf(msg)
		a.logError(err, msg, layer, "output", json, "groupVersionKind", gvk, "object", obj)
		return nil, nil, err
	}
}

func (a *KubectlLayerApplier) decodeList(layer layers.Layer,
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

func layerLabel(layer layers.Layer) string {
	return fmt.Sprintf("%s/%s", layer.GetNamespace(), layer.GetName())
}

// Apply an AddonLayer to the cluster.
func (a *KubectlLayerApplier) Apply(layer layers.Layer) (err error) {
	sourceDir := layer.GetSourcePath()
	a.logInfo("Applying AddonsLayer", layer)
	info, err := os.Stat(sourceDir)
	if os.IsNotExist(err) {
		a.logDebug("source directory not found", layer)
		return fmt.Errorf("source directory (%s) not found for AddonsLayer %s", sourceDir, layerLabel(layer))
	}
	if os.IsPermission(err) {
		a.logDebug("source directory read permission denied", layer)
		return fmt.Errorf("read permission denied to source directory (%s) for AddonsLayer %s", sourceDir, layerLabel(layer))
	}
	if err != nil {
		a.logError(err, "error while checking source directory", layer)
		return fmt.Errorf("error while checking source directory (%s) for AddonsLayer %s", sourceDir, layerLabel(layer))
	}
	if !info.IsDir() {
		// I'm not sure if this is an error, but I thought I should detect and log it
		a.logInfo("source path is not a directory", layer)
	}

	output, err := a.kubectl.Apply(sourceDir).WithLogger(layer.GetLogger()).Run()
	if err != nil {
		return fmt.Errorf("error from kubectl while applying source directory (%s) for AddonsLayer %s", sourceDir, layerLabel(layer))
	}

	hrs, errz, err := a.decodeAddons(layer, output)
	if err != nil {
		if errz != nil && len(errz) > 0 {
			a.logErrors(errz, layer)
		}
		return err
	}

	hrClient := layer.GetHelmReleaseClient()

	// TODO: Add an ownerRef to each deployed HelmRelease the points back to this AddonsLayer (needed for Prune)

	// TODO: Add timeout for watching addons
	tracker := a.newWatcher(layer, hrClient, hrs, 4*time.Minute).watchHrs()

	// TODO - We could return the tracker if the controller wants more information about what happened
	if tracker.releaseFailed() {
		tracker.logReleaseErrors(a, layer)
		return fmt.Errorf("release failed for AddonsLayer %s", layerLabel(layer))
	}

	return nil
}

// Prune the AddonsLayer by removing the Addons found in the cluster that have since been removed from the Layer.
func (a *KubectlLayerApplier) Prune(layer layers.Layer) (err error) {
	// TODO: Stub method placeholder.
	// Get a list of HelmRelease resources described in the YAML files in the layer's sourceDirectory from the output of kubectl apply -R -f <sourceDir> --dry-run -o json
	// Get a list of HelmRelease resources in the cluster with ownerRefs to this AddonsLayer
	// Remove all HelmRelease resources found in the sourceDirectory from the list of HelmRelease resources in the cluster
	// Delete all HelmRelease resources in the remaining list from the cluster
	// Wait until Helm Operator finishes removing all deleted HelmRelease resources from the cluster (respect the timeout)
	// Return an error if any HelmRelease resources could not be deleted or the timeout has been exceeded
	return nil
}
