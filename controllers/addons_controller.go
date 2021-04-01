/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"time"

	helmctlv2 "github.com/fluxcd/helm-controller/api/v2beta1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	kscheme "k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	kraanv1alpha1 "github.com/fidelity/kraan/api/v1alpha1"
	"github.com/fidelity/kraan/pkg/apply"
	"github.com/fidelity/kraan/pkg/common"
	"github.com/fidelity/kraan/pkg/layers"
	"github.com/fidelity/kraan/pkg/logging"
	"github.com/fidelity/kraan/pkg/metrics"
	"github.com/fidelity/kraan/pkg/repos"
)

var (
	hrOwnerKey = ".owner"
	reconciler *AddonsLayerReconciler
)

const (
	reasonRegex = "^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$" // Regex for k8s 1.19 conditions reason field
)

// AddonsLayerReconcilerOptions are the reconciller options
type AddonsLayerReconcilerOptions struct {
	MaxConcurrentReconciles int
}

// SetupWithManagerAndOptions setup manager with supplied options
func (r *AddonsLayerReconciler) SetupWithManagerAndOptions(mgr ctrl.Manager, opts AddonsLayerReconcilerOptions) error { // nolint: funlen,gocyclo,gocognit // ok
	logging.TraceCall(r.Log)
	defer logging.TraceExit(r.Log)

	addonsLayer := &kraanv1alpha1.AddonsLayer{}
	hr := &helmctlv2.HelmRelease{}
	hrepo := &sourcev1.HelmRepository{}

	if err := mgr.GetFieldIndexer().IndexField(r.Context, &helmctlv2.HelmRelease{}, hrOwnerKey, r.indexHelmReleaseByOwner); err != nil {
		return errors.Wrap(err, "failed setting up FieldIndexer for HelmRelease owner")
	}

	if err := mgr.GetFieldIndexer().IndexField(r.Context, &sourcev1.HelmRepository{}, hrOwnerKey, r.indexHelmRepoByOwner); err != nil {
		return errors.Wrap(err, "failed setting up FieldIndexer for HelmRepository owner")
	}

	ctl, err := ctrl.NewControllerManagedBy(mgr).
		For(addonsLayer).
		Owns(hr).
		Owns(hrepo).
		WithOptions(controller.Options{MaxConcurrentReconciles: opts.MaxConcurrentReconciles}).
		WithEventFilter(predicates(r.Log)).
		Build(r)
	if err != nil {
		return errors.Wrap(err, "error creating controller")
	}
	err = ctl.Watch(
		&source.Kind{Type: &sourcev1.GitRepository{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(r.repoMapperFunc),
		},
		predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				r.Log.V(1).Info("create event for GitRepository", append(logging.GetFunctionAndSource(logging.MyCaller), logging.GetObjKindNamespaceName(e.Object)...)...)
				return true
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				r.Log.V(1).Info("update event", append(logging.GetFunctionAndSource(logging.MyCaller), logging.GetObjKindNamespaceName(e.ObjectNew)...)...)
				if diff := cmp.Diff(e.MetaOld, e.MetaNew); len(diff) > 0 {
					r.Log.V(1).Info("update event meta change", append(logging.GetFunctionAndSource(logging.MyCaller), append(logging.GetObjKindNamespaceName(e.ObjectNew), "diff", diff)...)...)
				}
				if diff := cmp.Diff(e.ObjectOld, e.ObjectNew); len(diff) > 0 {
					r.Log.V(1).Info("update event object change", append(logging.GetFunctionAndSource(logging.MyCaller), append(logging.GetObjKindNamespaceName(e.ObjectNew), "diff", diff)...)...)
				}
				if e.MetaOld == nil || e.MetaNew == nil {
					r.Log.Error(fmt.Errorf("nill object passed to watcher"), "skipping processing",
						append(logging.GetFunctionAndSource(logging.MyCaller), "data", logging.LogJSON(e))...)
					return false
				}

				oldRepo, ok := e.ObjectOld.(*sourcev1.GitRepository)
				if !ok {
					r.Log.Error(fmt.Errorf("unable to cast old object to GitRepository"), "skipping processing",
						append(logging.GetFunctionAndSource(logging.MyCaller), "data", logging.LogJSON(e))...)
					return false
				}

				newRepo, ok := e.ObjectNew.(*sourcev1.GitRepository)
				if !ok {
					r.Log.Error(fmt.Errorf("unable to cast new object to GitRepository"), "skipping processing",
						append(logging.GetFunctionAndSource(logging.MyCaller), "data", logging.LogJSON(e))...)
					return false
				}

				if oldRepo.GetArtifact() == nil && newRepo.GetArtifact() != nil {
					r.Log.V(1).Info("new revision to process",
						append(logging.GetFunctionAndSource(logging.MyCaller), logging.GetGitRepoInfo(newRepo)...)...)
					return true
				}

				if oldRepo.GetArtifact() != nil && newRepo.GetArtifact() != nil &&
					oldRepo.GetArtifact().Revision != newRepo.GetArtifact().Revision {
					r.Log.V(1).Info("changed revision to process", logging.GetGitRepoInfo(newRepo)...)
					r.Log.V(1).Info("old revision", logging.GetGitRepoInfo(oldRepo)...)
					return true
				}
				repo := r.Repos.Add(newRepo)
				if repo.IsSynced() {
					r.Log.V(1).Info("no change to revision, but not yet synced",
						append(logging.GetFunctionAndSource(logging.MyCaller), logging.GetGitRepoInfo(newRepo)...)...)
					return true
				}

				r.Log.V(1).Info("no change to revision, not processing",
					append(logging.GetFunctionAndSource(logging.MyCaller), logging.GetGitRepoInfo(newRepo)...)...)
				return false
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				r.Log.V(1).Info("delete event for GitRepository", append(logging.GetFunctionAndSource(logging.MyCaller), logging.GetObjKindNamespaceName(e.Object))...)
				srcRepo, ok := e.Object.(*sourcev1.GitRepository)
				if !ok {
					r.Log.Error(fmt.Errorf("unable to cast deleted object to GitRepository"), "skipping processing",
						append(logging.GetFunctionAndSource(logging.MyCaller), "data", logging.LogJSON(e))...)
					return false
				}
				r.Repos.Delete(repos.PathKey(srcRepo))
				r.Log.V(1).Info("delete repo object", logging.GetGitRepoInfo(srcRepo)...)
				return false
			},
			GenericFunc: func(e event.GenericEvent) bool {
				r.Log.V(1).Info("generic event for GitRepository", append(logging.GetFunctionAndSource(logging.MyCaller), logging.GetObjKindNamespaceName(e.Object))...)
				return true
			},
		},
	)
	if err != nil {
		return errors.Wrap(err, "error creating controller")
	}
	err = ctl.Watch(
		&source.Kind{Type: &kraanv1alpha1.AddonsLayer{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(r.layerMapperFunc),
		},
		predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				r.Log.V(1).Info("create event for AddonsLayer, not processing will be processed by controller reconciler",
					append(logging.GetFunctionAndSource(logging.MyCaller), logging.GetObjKindNamespaceName(e.Object)...)...)
				return false
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				r.Log.V(1).Info("update event for AddonsLayer", append(logging.GetFunctionAndSource(logging.MyCaller), logging.GetObjKindNamespaceName(e.ObjectNew)...)...)
				if diff := cmp.Diff(e.MetaOld, e.MetaNew); len(diff) > 0 {
					r.Log.V(1).Info("update event meta change for AddonsLayer",
						append(logging.GetFunctionAndSource(logging.MyCaller), append(logging.GetObjKindNamespaceName(e.ObjectNew), "diff", diff)...)...)
				}
				if diff := cmp.Diff(e.ObjectOld, e.ObjectNew); len(diff) > 0 {
					r.Log.V(1).Info("update event object change for AddonsLayer",
						append(logging.GetFunctionAndSource(logging.MyCaller), append(logging.GetObjKindNamespaceName(e.ObjectNew), "diff", diff)...)...)
				}
				if e.MetaOld == nil || e.MetaNew == nil {
					r.Log.Error(fmt.Errorf("nill object passed to watcher for AddonsLayer"), "skipping processing",
						append(logging.GetFunctionAndSource(logging.MyCaller), "data", logging.LogJSON(e))...)
					return false
				}

				old, ok := e.ObjectOld.(*kraanv1alpha1.AddonsLayer)
				if !ok {
					r.Log.Error(fmt.Errorf("unable to cast old object to AddonsLayer"), "skipping processing",
						append(logging.GetFunctionAndSource(logging.MyCaller), "data", logging.LogJSON(e))...)
					return false
				}
				new, ok := e.ObjectNew.(*kraanv1alpha1.AddonsLayer)
				if !ok {
					r.Log.Error(fmt.Errorf("unable to cast new object to AddonsLayer"), "skipping processing",
						append(logging.GetFunctionAndSource(logging.MyCaller), "data", logging.LogJSON(e))...)
					return false
				}

				if common.GetSourceNamespace(old.Spec.Source.NameSpace) != common.GetSourceNamespace(new.Spec.Source.NameSpace) || old.Spec.Source.Name != new.Spec.Source.Name {
					r.Log.V(1).Info("layer source changed, remove from users list for previous source",
						append(logging.GetFunctionAndSource(logging.MyCaller), logging.GetLayerInfo(new)...)...)
					repoName := fmt.Sprintf("%s/%s", common.GetSourceNamespace(old.Spec.Source.NameSpace), old.Spec.Source.Name)
					repo := r.Repos.Get(repoName)
					if repo != nil {
						if repo.RemoveUser(old.Name) {
							// Last user
							r.Repos.Delete(repoName)
						}
					}
				}

				if new.Status.State == kraanv1alpha1.DeployedCondition {
					r.Log.V(1).Info("layer deployed, process dependent layers",
						append(logging.GetFunctionAndSource(logging.MyCaller), logging.GetLayerInfo(new)...)...)
					return true
				}
				r.Log.V(1).Info("layer status not yet deployed, not processing",
					append(logging.GetFunctionAndSource(logging.MyCaller), logging.GetLayerInfo(new)...)...)
				return false
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				r.Log.V(1).Info("delete event for AddonsLayer, not processing will be processed by controller reconciler",
					append(logging.GetFunctionAndSource(logging.MyCaller), logging.GetObjKindNamespaceName(e.Object)...)...)
				return false
			},
			GenericFunc: func(e event.GenericEvent) bool {
				r.Log.V(1).Info("generic event for AddonsLayer, not processing will be processed by controller reconciler",
					append(logging.GetFunctionAndSource(logging.MyCaller), logging.GetObjKindNamespaceName(e.Object)...)...)
				return false
			},
		},
	)
	return err
}

func predicates(logger logr.Logger) predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			logger.V(1).Info("create event", append(logging.GetFunctionAndSource(logging.MyCaller),
				append(logging.GetObjKindNamespaceName(e.Object), "layer", logging.GetLayer(e.Object))...)...)
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			logger.V(1).Info("update event", append(logging.GetFunctionAndSource(logging.MyCaller),
				append(logging.GetObjKindNamespaceName(e.ObjectNew), "layer", logging.GetLayer(e.ObjectNew))...)...)
			if diff := cmp.Diff(e.MetaOld, e.MetaNew); len(diff) > 0 {
				logger.V(1).Info("update event meta change", append(logging.GetFunctionAndSource(logging.MyCaller), append(logging.GetObjKindNamespaceName(e.ObjectNew), "diff", diff)...)...)
			}
			if diff := cmp.Diff(e.ObjectOld, e.ObjectNew); len(diff) > 0 {
				logger.V(1).Info("update event object change", append(logging.GetFunctionAndSource(logging.MyCaller), append(logging.GetObjKindNamespaceName(e.ObjectNew), "diff", diff)...)...)
			}
			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			logger.V(1).Info("generic event", append(logging.GetFunctionAndSource(logging.MyCaller),
				append(logging.GetObjKindNamespaceName(e.Object), "layer", logging.GetLayer(e.Object))...)...)
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			logger.V(1).Info("delete event", append(logging.GetFunctionAndSource(logging.MyCaller),
				append(logging.GetObjKindNamespaceName(e.Object), "layer", logging.GetLayer(e.Object))...)...)
			return true
		},
	}
}

// AddonsLayerReconciler reconciles a AddonsLayer object.
type AddonsLayerReconciler struct {
	client.Client
	Config   *rest.Config
	k8client kubernetes.Interface
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Context  context.Context
	Applier  apply.LayerApplier
	Repos    repos.Repos
	Metrics  metrics.Metrics
	Recorder record.EventRecorder
	regex    *regexp.Regexp
}

// EventRecorder returns an EventRecorder type that can be
// used to post Events to different object's lifecycles.
func eventRecorder(
	kubeClient kubernetes.Interface) record.EventRecorder {
	eventBroadcaster := record.NewBroadcaster()
	//eventBroadcaster.StartLogging(setupLog.Infof)
	eventBroadcaster.StartRecordingToSink(
		&typedcorev1.EventSinkImpl{
			Interface: kubeClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(
		kscheme.Scheme,
		corev1.EventSource{Component: "kraan-controller"})
	return recorder
}

// NewReconciler returns an AddonsLayerReconciler instance
func NewReconciler(config *rest.Config, client client.Client, logger logr.Logger,
	scheme *runtime.Scheme) (*AddonsLayerReconciler, error) {
	reconciler = &AddonsLayerReconciler{
		Config: config,
		Client: client,
		Log:    logger.WithName("reconciler"),
		Scheme: scheme,
	}
	var err error
	reconciler.k8client, err = reconciler.getK8sClient()
	if err != nil {
		return nil, errors.WithMessagef(err, "%s - failed to create reconciler", logging.CallerStr(logging.Me))
	}
	reconciler.Recorder = eventRecorder(reconciler.k8client)
	reconciler.Context = context.Background()
	reconciler.Applier, err = apply.NewApplier(client, logger.WithName("applier"), scheme)
	if err != nil {
		return nil, errors.WithMessagef(err, "%s - failed to create applier", logging.CallerStr(logging.Me))
	}
	reconciler.Repos = repos.NewRepos(reconciler.Context, reconciler.Log)

	reconciler.Metrics = metrics.NewMetrics()

	reconciler.regex, err = regexp.Compile(reasonRegex)
	if err != nil {
		return nil, errors.Wrapf(err, "%s - failed to compile regex", logging.CallerStr(logging.Me))
	}
	return reconciler, err
}

func (r *AddonsLayerReconciler) getK8sClient() (kubernetes.Interface, error) {
	logging.TraceCall(r.Log)
	defer logging.TraceExit(r.Log)

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(r.Config)
	if err != nil {
		return nil, errors.Wrapf(err, "%s - failed to create k8s client", logging.CallerStr(logging.Me))
	}

	return clientset, nil
}

func (r *AddonsLayerReconciler) orphans(l layers.Layer, hrs []*helmctlv2.HelmRelease) (bool, error) {
	if len(hrs) == 0 {
		return false, nil
	}

	orphansPending := false

	for _, hr := range hrs {
		orphan, err := r.Applier.Orphan(r.Context, l, hr)
		if err != nil {
			return true, errors.WithMessagef(err, "%s - check for orphans failed", logging.CallerStr(logging.Me))
		}

		orphansPending = orphansPending || orphan
	}

	return orphansPending, nil
}

func (r *AddonsLayerReconciler) processPrune(l layers.Layer) (statusReconciled bool, err error) {
	logging.TraceCall(r.Log)
	defer logging.TraceExit(r.Log)

	ctx := r.Context
	applier := r.Applier

	pruneIsRequired, hrs, err := applier.PruneIsRequired(ctx, l)
	if err != nil {
		return false, errors.WithMessagef(err, "%s - check for apply required failed", logging.CallerStr(logging.Me))
	}

	if pruneIsRequired {
		l.SetStatusPruning()

		orphans, orphanErr := r.orphans(l, hrs)
		if orphanErr != nil {
			return true, errors.WithMessagef(orphanErr, "%s - orphan check failed", logging.CallerStr(logging.Me))
		}

		if orphans { // Outstanding orphans waiting to be adopted
			l.SetDelayedRequeue() // Schedule requeue
			return true, nil      // don't proceed with prune
		}

		if pruneErr := applier.Prune(ctx, l, hrs); pruneErr != nil {
			return true, errors.WithMessagef(pruneErr, "%s - prune failed", logging.CallerStr(logging.Me))
		}
		l.SetDelayedRequeue()
		return true, nil
	}

	return false, nil
}

func (r *AddonsLayerReconciler) processApply(l layers.Layer) (statusReconciled bool, err error) {
	logging.TraceCall(r.Log)
	defer logging.TraceExit(r.Log)

	ctx := r.Context
	applier := r.Applier

	if !l.DependenciesDeployed() {
		l.SetRequeue()
		return true, nil
	}

	applyIsRequired, err := applier.ApplyIsRequired(ctx, l)
	if err != nil {
		return false, errors.WithMessagef(err, "%s - check if apply is required failed", logging.CallerStr(logging.Me))
	}
	if applyIsRequired {
		if !l.DependenciesDeployed() {
			l.SetDelayedRequeue()
			return true, nil
		}

		l.SetStatusApplying()
		if applyErr := applier.Apply(ctx, l); applyErr != nil {
			return true, errors.WithMessagef(applyErr, "%s - apply failed", logging.CallerStr(logging.Me))
		}
		l.SetDelayedRequeue()
		return true, nil
	}
	return false, nil
}

func (r *AddonsLayerReconciler) checkPruneFailed(l layers.Layer) {
	if l.GetStatus() != kraanv1alpha1.PruningCondition {
		return
	}
	timeoutTime := l.GetFullStatus().Conditions[0].LastTransitionTime.Add(l.GetTimeout())
	if metav1.Now().Time.Before(timeoutTime) {
		return
	}
	l.StatusUpdate(kraanv1alpha1.FailedCondition, "pruning not complete after timeout period")
}

func (r *AddonsLayerReconciler) setHelmReleaseFailed(l layers.Layer, hrName string) {
	if l.GetStatus() == kraanv1alpha1.ApplyingCondition {
		timeoutTime := l.GetFullStatus().Conditions[0].LastTransitionTime.Add(l.GetTimeout())
		if metav1.Now().Time.Before(timeoutTime) {
			return
		}
	}
	r.Log.Info("HelmRelease not deployed", append(logging.GetFunctionAndSource(logging.MyCaller), "layer", l.GetName(), "name", hrName)...)
	l.StatusUpdate(kraanv1alpha1.FailedCondition, fmt.Sprintf("%s, HelmRelease: %s, not ready", kraanv1alpha1.AddonsLayerFailedMsg, hrName))
}

func (r *AddonsLayerReconciler) checkSuccess(l layers.Layer) (string, error) {
	logging.TraceCall(r.Log)
	defer logging.TraceExit(r.Log)

	ctx := r.Context
	applier := r.Applier

	applyWasSuccessful, hrName, err := applier.ApplyWasSuccessful(ctx, l)
	if err != nil {
		return "", errors.WithMessagef(err, "%s - check for apply required failed", logging.CallerStr(logging.Me))
	}
	if !applyWasSuccessful {
		r.setHelmReleaseFailed(l, hrName)
		l.SetDelayedRequeue()
		return "", nil
	}
	l.SetStatusDeployed()
	revision, err := r.getRevision(l)
	if err != nil {
		return "", errors.WithMessagef(err, "%s - failed to get revision", logging.CallerStr(logging.Me))
	}
	return revision, nil
}

func (r *AddonsLayerReconciler) waitForData(l layers.Layer, repo repos.Repo) (err error) {
	logging.TraceCall(r.Log)
	defer logging.TraceExit(r.Log)

	MaxTries := 15
	for try := 1; try < MaxTries; try++ {
		err = repo.LinkData(l.GetSourcePath(), l.GetSpec().Source.Path)
		if err == nil {
			r.Log.V(1).Info("linked to layer data",
				append(logging.GetFunctionAndSource(logging.MyCaller), "requestName", l.GetName(), "kind", logging.GitRepoSourceKind(),
					"namespace", common.GetSourceNamespace(l.GetSpec().Source.NameSpace), "name", l.GetSpec().Source.Name, "layer", l.GetName())...)
			return nil
		}
		r.Log.V(1).Info("waiting for layer data to be synced",
			append(logging.GetFunctionAndSource(logging.MyCaller), "layer", l.GetName(), "kind", logging.GitRepoSourceKind(),
				"namespace", common.GetSourceNamespace(l.GetSpec().Source.NameSpace), "name", l.GetSpec().Source.Name, "path", l.GetSpec().Source.Path)...)
		time.Sleep(time.Second)
	}
	l.StatusUpdate(kraanv1alpha1.FailedCondition, fmt.Sprintf("%s, %s", kraanv1alpha1.AddonsLayerFailedMsg, errors.Cause(err).Error()))
	return errors.WithMessagef(err, "%s - failed to link to layer data", logging.CallerStr(logging.Me))
}

func (r *AddonsLayerReconciler) checkData(l layers.Layer) (bool, error) {
	logging.TraceCall(r.Log)
	defer logging.TraceExit(r.Log)

	sourceRepoName := l.GetSourceKey()
	MaxTries := 5
	for try := 1; try < MaxTries; try++ {
		repo := r.Repos.Get(sourceRepoName)
		if repo != nil {
			if err := r.waitForData(l, repo); err != nil {
				return false, errors.WithMessagef(err, "%s - failed to find layer data", logging.CallerStr(logging.Me))
			}
			return true, nil
		}
		r.Log.Info("waiting for layer data",
			append(logging.GetFunctionAndSource(logging.MyCaller), "requestName", l.GetName(), "kind", logging.GitRepoSourceKind(), "source", l.GetSpec().Source)...)
		time.Sleep(time.Duration(time.Second * time.Duration(try))) // nolint: unconvert // ignore
	}
	l.SetDelayedRequeue()
	l.SetStatusPending()
	return false, nil
}

func (r *AddonsLayerReconciler) getRevision(l layers.Layer) (string, error) {
	logging.TraceCall(r.Log)
	defer logging.TraceExit(r.Log)

	sourceRepoName := l.GetSourceKey()
	repo := r.Repos.Get(sourceRepoName)
	if repo == nil {
		return "", fmt.Errorf("unable to find repo object")
	}
	return repo.GetGitRepo().Status.Artifact.Revision, nil
}

func (r *AddonsLayerReconciler) compareResources(current, new []kraanv1alpha1.Resource) bool {
	logging.TraceCall(r.Log)
	defer logging.TraceExit(r.Log)

	if len(current) != len(new) {
		return false
	}

	sort.Sort(kraanv1alpha1.Resources(current))
	sort.Sort(kraanv1alpha1.Resources(new))
	for index, resource := range current {
		if !apply.CompareAsJSON(resource, new[index]) {
			return false
		}
	}
	return true
}

func (r *AddonsLayerReconciler) updateResources(l layers.Layer) {
	logging.TraceCall(r.Log)
	defer logging.TraceExit(r.Log)

	ctx := r.Context
	applier := r.Applier

	resources, err := applier.GetResources(ctx, l)
	if err != nil {
		r.Log.Error(err, "failed to get resources", logging.GetFunctionAndSource(logging.MyCaller)...)
		return
	}
	if !r.compareResources(l.GetFullStatus().Resources, resources) {
		sort.Sort(kraanv1alpha1.Resources(resources))
		l.GetFullStatus().Resources = resources
		l.SetUpdated()
		r.Log.V(3).Info("updated resources", append(logging.GetFunctionAndSource(logging.MyCaller), "layer", l.GetName(), "resources", l.GetFullStatus().Resources)...)
	}
}

func (r *AddonsLayerReconciler) isReady(l layers.Layer) (bool, error) {
	logging.TraceCall(r.Log)
	defer logging.TraceExit(r.Log)

	sourceRepoName := l.GetSourceKey()
	repo := r.Repos.Get(sourceRepoName)
	if repo == nil {
		return false, fmt.Errorf("unable to find git repository object")
	}
	revision := "not set"
	if repo.GetGitRepo().Status.Artifact != nil {
		revision = repo.GetGitRepo().Status.Artifact.Revision
	}
	ready, srcMsg := l.RevisionReady(repo.GetGitRepo().Status.Conditions, revision)
	if !ready {
		l.SetDelayedRequeue()
		message := fmt.Sprintf("layer source: %s not ready, source state: %s.", repo.GetGitRepo().Name, srcMsg)
		l.StatusUpdate(kraanv1alpha1.PendingCondition, message)
		return false, nil
	}
	return true, nil
}

func (r *AddonsLayerReconciler) adopt(l layers.Layer) error {
	orphanedHrs, err := r.Applier.GetOrphanedHelmReleases(r.Context, l)
	if err != nil {
		return errors.WithMessagef(err, "%s - failed to get orphaned helm releases", logging.CallerStr(logging.Me))
	}

	sourceHrs, _, err := r.Applier.GetSourceAndClusterHelmReleases(r.Context, l)
	if err != nil {
		return errors.WithMessagef(err, "%s - failed to get helm releases", logging.CallerStr(logging.Me))
	}

	for hrName, hr := range orphanedHrs {
		if _, ok := sourceHrs[hrName]; ok { // Orphaned HelmRelease is in our source
			err := r.Applier.Adopt(r.Context, l, hr)
			if err != nil {
				return errors.WithMessagef(err, "%s - failed to adopt helm releases", logging.CallerStr(logging.Me))
			}
		}
	}
	return nil
}

func (r *AddonsLayerReconciler) processAddonLayer(l layers.Layer) (string, error) { // nolint: gocyclo // ok
	logging.TraceCall(r.Log)
	defer logging.TraceExit(r.Log)

	l.GetLogger().Info("processing", logging.GetFunctionAndSource(logging.MyCaller)...)

	if l.IsHold() {
		l.SetHold()
		return "", nil
	}

	ready, err := r.isReady(l)
	if err != nil {
		return "", errors.WithMessagef(err, "%s - failed to check source is ready", logging.CallerStr(logging.Me))
	}
	if !ready {
		return "", nil
	}

	layerDataReady, err := r.checkData(l)
	if err != nil {
		return "", errors.WithMessagef(err, "%s - failed layer data is not ready", logging.CallerStr(logging.Me))
	}
	if !layerDataReady {
		return "", nil
	}

	defer r.updateResources(l)

	err = r.adopt(l)
	if err != nil {
		return "", errors.WithMessagef(err, "%s - failed to perform adopt processing", logging.CallerStr(logging.Me))
	}

	if !l.CheckK8sVersion() {
		l.SetStatusK8sVersion()
		l.SetDelayedRequeue()
		return "", nil
	}

	layerStatusUpdated, err := r.processPrune(l)
	if err != nil {
		return "", errors.WithMessagef(err, "%s - failed to perform prune processing", logging.CallerStr(logging.Me))
	}
	if layerStatusUpdated {
		return "", nil
	}

	r.checkPruneFailed(l)

	layerStatusUpdated, err = r.processApply(l)
	if err != nil {
		return "", errors.WithMessagef(err, "%s - failed to perform apply processing", logging.CallerStr(logging.Me))
	}
	if layerStatusUpdated {
		return "", nil
	}

	return r.checkSuccess(l)
}

func (r *AddonsLayerReconciler) updateRequeue(l layers.Layer) (res ctrl.Result, rerr error) {
	logging.TraceCall(r.Log)
	defer logging.TraceExit(r.Log)

	if l.IsUpdated() {
		if rerr = r.update(r.Context, l.GetAddonsLayer()); rerr != nil {
			return ctrl.Result{Requeue: true}, errors.WithMessagef(rerr, "%s - failed to update", logging.CallerStr(logging.Me))
		}
	}
	if l.NeedsRequeue() {
		if l.IsDelayed() {
			r.Log.V(1).Info("delayed requeue", append(logging.GetFunctionAndSource(logging.MyCaller), "layer", l.GetName(), "interval", l.GetDelay())...)
			res = ctrl.Result{Requeue: true, RequeueAfter: l.GetDelay()}
			return res, nil
		}
		r.Log.V(1).Info("requeue", append(logging.GetFunctionAndSource(logging.MyCaller), "layer", l.GetName())...)
		res = ctrl.Result{Requeue: true}
		return res, nil
	}
	return ctrl.Result{}, nil
}

// Reconcile process AddonsLayers custom resources.
// +kubebuilder:rbac:groups=kraan.io,resources=addons,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kraan.io,resources=addons/status,verbs=get;update;patch
func (r *AddonsLayerReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) { // nolint:funlen,gocyclo,gocognit // ok
	logging.TraceCall(r.Log)
	defer logging.TraceExit(r.Log)

	ctx := r.Context
	reconcileStart := time.Now()

	var addonsLayer *kraanv1alpha1.AddonsLayer = &kraanv1alpha1.AddonsLayer{}
	if err := r.Get(ctx, req.NamespacedName, addonsLayer); err != nil {
		if apierrors.IsNotFound(err) {
			r.Log.Info("addonsLayer deleted", append(logging.GetFunctionAndSource(logging.MyCaller), "layer", req.NamespacedName.Name)...)
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	r.Log.V(2).Info("addonsLayer data", append(logging.GetFunctionAndSource(logging.MyCaller), "layer", req.NamespacedName.Name, "data", logging.LogJSON(addonsLayer))...)

	log := r.Log.WithValues("layer", req.NamespacedName.Name)

	l := layers.CreateLayer(ctx, r.Client, r.k8client, log, r.Recorder, r.Scheme, addonsLayer)

	// Following deletion and finalizer handling copied from HelmController
	/*
		Copyright 2020 The Flux CD contributors.

		Licensed under the Apache License, Version 2.0 (the "License");
		you may not use this file except in compliance with the License.
		You may obtain a copy of the License at

			http://www.apache.org/licenses/LICENSE-2.0

		Unless required by applicable law or agreed to in writing, software
		distributed under the License is distributed on an "AS IS" BASIS,
		WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
		See the License for the specific language governing permissions and
		limitations under the License.
	*/

	updated, res, err := r.upgradeFix(ctx, addonsLayer)
	if updated {
		return res, err
	}

	if addonsLayer.ObjectMeta.DeletionTimestamp.IsZero() { // nolint: nestif // ok
		if !common.ContainsString(addonsLayer.ObjectMeta.Finalizers, kraanv1alpha1.AddonsFinalizer) {
			r.Log.V(1).Info("adding finalizer to addonsLayer", append(logging.GetFunctionAndSource(logging.MyCaller), "layer", req.NamespacedName.Name)...)
			addonsLayer.ObjectMeta.Finalizers = append(l.GetAddonsLayer().ObjectMeta.Finalizers, kraanv1alpha1.AddonsFinalizer)
			if err = r.Update(ctx, l.GetAddonsLayer()); err != nil {
				r.Log.Error(err, "unable to register finalizer", append(logging.GetFunctionAndSource(logging.MyCaller), "layer", req.NamespacedName.Name)...)
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
	} else {
		r.Log.Info("addonsLayer is being deleted", append(logging.GetFunctionAndSource(logging.MyCaller), "layer", req.NamespacedName.Name)...)
		if common.ContainsString(addonsLayer.ObjectMeta.Finalizers, kraanv1alpha1.AddonsFinalizer) {
			clusterHrs, e := r.Applier.GetHelmReleases(ctx, l)
			if e != nil {
				r.Log.Error(e, "failed to get helm resources", append(logging.GetFunctionAndSource(logging.MyCaller), "layer", req.NamespacedName.Name)...)
				return ctrl.Result{}, errors.WithMessagef(err, "%s - failed to get helm releases", logging.CallerStr(logging.Me))
			}

			hrs := make([]*helmctlv2.HelmRelease, 0, len(clusterHrs))

			for _, value := range clusterHrs {
				hrs = append(hrs, value)
			}

			// label HRs owned by this layer as orphaned if not already labelled, set label value to time now.
			orphans, orphanErr := r.orphans(l, hrs)
			if orphanErr != nil {
				return ctrl.Result{}, errors.WithMessagef(orphanErr, "%s - orphan check failed", logging.CallerStr(logging.Me))
			}

			if orphans { // Outstanding orphans waiting to be adopted
				r.Log.Info("addonsLayer waiting for adoption", append(logging.GetFunctionAndSource(logging.MyCaller), "layer", req.NamespacedName.Name)...)
				l.SetDelayedRequeue()     // Schedule requeue
				return r.updateRequeue(l) // don't proceed with delete
			}
			// Remove our finalizer from the list and update it
			addonsLayer.ObjectMeta.Finalizers = common.RemoveString(l.GetAddonsLayer().ObjectMeta.Finalizers, kraanv1alpha1.AddonsFinalizer)
			r.recordReadiness(addonsLayer, true)
			l.SetDeleted()
			r.Log.Info("addonsLayer deleted", append(logging.GetFunctionAndSource(logging.MyCaller), "layer", req.NamespacedName.Name)...)
			if err = r.Update(ctx, l.GetAddonsLayer()); err != nil {
				r.Log.Error(err, "unable to remove finalizer", append(logging.GetFunctionAndSource(logging.MyCaller), "layer", req.NamespacedName.Name)...)
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	deployedRevision, err := r.processAddonLayer(l)
	if err != nil {
		l.StatusUpdate(kraanv1alpha1.FailedCondition, fmt.Sprintf("%s, %s", kraanv1alpha1.AddonsLayerFailedMsg, errors.Cause(err).Error()))
		log.Error(err, "failed to process addons layer", logging.GetFunctionAndSource(logging.MyCaller)...)
	}

	if l.GetSpec().Version != l.GetFullStatus().Version {
		l.SetUpdated()
		l.GetFullStatus().Version = l.GetSpec().Version
	}

	if l.GetAddonsLayer().Generation != l.GetFullStatus().ObservedGeneration {
		l.SetUpdated()
		l.GetFullStatus().ObservedGeneration = l.GetAddonsLayer().Generation
	}
	if len(deployedRevision) > 0 && deployedRevision != l.GetFullStatus().DeployedRevision {
		l.SetUpdated()
		l.GetFullStatus().DeployedRevision = deployedRevision
	}

	r.Metrics.RecordDuration(l.GetAddonsLayer(), reconcileStart)
	r.recordReadiness(l.GetAddonsLayer(), false)

	r.Log.V(2).Info("addonsLayer data", append(logging.GetFunctionAndSource(logging.MyCaller), "layer", req.NamespacedName.Name, "data", logging.LogJSON(l.GetAddonsLayer()))...)
	return r.updateRequeue(l)
}

func (r *AddonsLayerReconciler) upgradeFix(ctx context.Context, al *kraanv1alpha1.AddonsLayer) (updated bool, res ctrl.Result, err error) {
	r.Log.V(1).Info("Checking existing conditions", append(logging.GetFunctionAndSource(logging.MyCaller), "layer", al.Name)...)
	for _, condition := range al.Status.Conditions {
		if condition.Status == metav1.ConditionFalse {
			continue
		}
		if !r.regex.MatchString(condition.Reason) {
			newCondition := metav1.Condition{}
			newCondition.LastTransitionTime = condition.LastTransitionTime
			newCondition.Status = condition.Status
			newCondition.Type = condition.Type
			newCondition.Reason = newCondition.Type
			newCondition.Message = fmt.Sprintf("%s, %s", condition.Reason, condition.Message)
			al.Status.Conditions = []metav1.Condition{newCondition}
			if al.Status.Resources == nil {
				al.Status.Resources = []kraanv1alpha1.Resource{}
			}
			if err := r.update(ctx, al); err != nil {
				r.Log.Error(err, "unable to update conditions", append(logging.GetFunctionAndSource(logging.MyCaller), "layer", al.Name)...)
				return true, ctrl.Result{}, err
			}
			return true, ctrl.Result{Requeue: true}, nil
		}
	}
	return false, ctrl.Result{}, nil
}

func (r *AddonsLayerReconciler) recordReadiness(al *kraanv1alpha1.AddonsLayer, deleted bool) {
	logging.TraceCall(r.Log)
	defer logging.TraceExit(r.Log)

	status := metav1.ConditionFalse
	if al.Status.State == kraanv1alpha1.DeployedCondition {
		status = metav1.ConditionTrue
	}
	r.Metrics.RecordCondition(al, metav1.Condition{
		Type:   kraanv1alpha1.DeployedCondition,
		Status: status,
	}, deleted)

	status = metav1.ConditionFalse
	if al.Status.State == kraanv1alpha1.FailedCondition {
		status = metav1.ConditionTrue
	}
	r.Metrics.RecordCondition(al, metav1.Condition{
		Type:   kraanv1alpha1.FailedCondition,
		Status: status,
	}, deleted)
}

func (r *AddonsLayerReconciler) update(ctx context.Context, a *kraanv1alpha1.AddonsLayer) error {
	logging.TraceCall(r.Log)
	defer logging.TraceExit(r.Log)
	r.Log.V(2).Info("addonsLayer data", append(logging.GetFunctionAndSource(logging.MyCaller), "layer", a.Name, "data", logging.LogJSON(a))...)
	if err := r.Status().Update(ctx, a); err != nil {
		r.Log.Error(err, "unable to update AddonsLayer status", logging.GetFunctionAndSource(logging.MyCaller)...)
		return errors.Wrapf(err, "%s - failed to update", logging.CallerStr(logging.Me))
	}

	return nil
}

func (r *AddonsLayerReconciler) repoMapperFunc(a handler.MapObject) []reconcile.Request { // nolint:gocyclo //ok
	logging.TraceCall(r.Log)
	defer logging.TraceExit(r.Log)

	srcRepo, ok := a.Object.(*sourcev1.GitRepository)
	if !ok {
		r.Log.Error(fmt.Errorf("unable to cast object to GitRepository"), "skipping processing", logging.GetObjKindNamespaceName(a.Object))
		return []reconcile.Request{}
	}

	r.Log.V(1).Info("monitoring", append(logging.GetGitRepoInfo(srcRepo), logging.GetFunctionAndSource(logging.MyCaller)...)...)
	addonsList := &kraanv1alpha1.AddonsLayerList{}
	if err := r.List(r.Context, addonsList); err != nil {
		r.Log.Error(err, "unable to list AddonsLayers", append(logging.GetGitRepoInfo(srcRepo), logging.GetFunctionAndSource(logging.MyCaller)...)...)
		return []reconcile.Request{}
	}
	layerList := []layers.Layer{}
	addons := []reconcile.Request{}
	for _, addon := range addonsList.Items {
		layer := layers.CreateLayer(r.Context, r.Client, r.k8client, r.Log, r.Recorder, r.Scheme, &addon) //nolint:scopelint // ok
		if layer.GetSpec().Source.Name == srcRepo.Name && common.GetSourceNamespace(layer.GetSpec().Source.NameSpace) == srcRepo.Namespace {
			r.Log.V(1).Info("layer is using this source", append(logging.GetGitRepoInfo(srcRepo), append(logging.GetFunctionAndSource(logging.MyCaller), "layer", addon.Name)...)...)
			layerList = append(layerList, layer)
			addons = append(addons, reconcile.Request{NamespacedName: types.NamespacedName{Name: layer.GetName(), Namespace: ""}})
		}
	}
	if len(addons) == 0 {
		return []reconcile.Request{}
	}
	repo := r.Repos.Add(srcRepo)
	r.Log.V(1).Info("created repo object", logging.GetGitRepoInfo(srcRepo)...)
	if err := repo.SyncRepo(); err != nil {
		r.Log.Error(err, "unable to sync repo, not requeuing", append(logging.GetGitRepoInfo(srcRepo), logging.GetFunctionAndSource(logging.MyCaller)...)...)
		return []reconcile.Request{}
	}

	for _, layer := range layerList {
		if err := repo.LinkData(layer.GetSourcePath(), layer.GetSpec().Source.Path); err != nil {
			r.Log.Error(err, "unable to link referencing AddonsLayer directory to repository data",
				append(logging.GetGitRepoInfo(srcRepo), append(logging.GetFunctionAndSource(logging.MyCaller), "layers", layer.GetName())...)...)
			continue
		}
		repo.AddUser(layer.GetName())
	}
	r.Log.V(1).Info("synced source", append(logging.GetGitRepoInfo(srcRepo), append(logging.GetFunctionAndSource(logging.MyCaller), "layers", addons)...)...)
	if err := repo.TidyRepo(); err != nil {
		r.Log.Error(err, "unable to garbage collect repo revisions", append(logging.GetGitRepoInfo(srcRepo), logging.GetFunctionAndSource(logging.MyCaller)...)...)
	}
	return addons
}

func (r *AddonsLayerReconciler) layerMapperFunc(a handler.MapObject) []reconcile.Request {
	logging.TraceCall(r.Log)
	defer logging.TraceExit(r.Log)

	src, ok := a.Object.(*kraanv1alpha1.AddonsLayer)
	if !ok {
		r.Log.Error(fmt.Errorf("unable to cast object to AddonsLayer"), "skipping processing", logging.GetObjKindNamespaceName(a.Object))
		return []reconcile.Request{}
	}

	r.Log.V(1).Info("monitoring", append(logging.GetLayerInfo(src), logging.GetFunctionAndSource(logging.MyCaller)...)...)
	addonsList := &kraanv1alpha1.AddonsLayerList{}
	if err := r.List(r.Context, addonsList); err != nil {
		r.Log.Error(err, "unable to list AddonsLayers", append(logging.GetLayerInfo(src), logging.GetFunctionAndSource(logging.MyCaller)...)...)
		return []reconcile.Request{}
	}
	addons := []reconcile.Request{}
	for _, addon := range addonsList.Items {
		layer := layers.CreateLayer(r.Context, r.Client, r.k8client, r.Log, r.Recorder, r.Scheme, &addon) //nolint:scopelint // ok
		if common.ContainsString(layer.GetSpec().PreReqs.DependsOn, fmt.Sprintf("%s@%s", src.Name, src.Status.Version)) {
			r.Log.V(1).Info("layer dependent on updated layer", append(logging.GetLayerInfo(src), append(logging.GetFunctionAndSource(logging.MyCaller), "layer", addon.Name)...)...)
			addons = append(addons, reconcile.Request{NamespacedName: types.NamespacedName{Name: layer.GetName(), Namespace: ""}})
		}
	}
	if len(addons) == 0 {
		return []reconcile.Request{}
	}
	return addons
}

func (r *AddonsLayerReconciler) indexHelmReleaseByOwner(o runtime.Object) []string {
	logging.TraceCall(r.Log)
	defer logging.TraceExit(r.Log)

	r.Log.V(2).Info("indexing", append(logging.GetObjKindNamespaceName(o), logging.GetFunctionAndSource(logging.MyCaller)...)...)
	hr, ok := o.(*helmctlv2.HelmRelease)
	if !ok {
		r.Log.Error(fmt.Errorf("failed to cast to helmrelease"), "failed to cast object to expected kind",
			append(logging.GetObjKindNamespaceName(o), logging.GetFunctionAndSource(logging.MyCaller)...)...)
		return nil
	}
	owner := metav1.GetControllerOf(hr)
	if owner == nil {
		r.Log.V(2).Info("unexpected helmrelease passed to indexer, no owner information",
			append(logging.GetFunctionAndSource(logging.MyCaller), "kind", "helmreleases.helm.toolkit.fluxcd.io", "namespace", hr.Namespace, "name", hr.Name)...)
		return nil
	}
	if owner.APIVersion != kraanv1alpha1.GroupVersion.String() || owner.Kind != "AddonsLayer" {
		r.Log.V(2).Info("unexpected helmrelease passed to indexer, not owned by an AddonsLayer",
			append(logging.GetFunctionAndSource(logging.MyCaller), "kind", "helmreleases.helm.toolkit.fluxcd.io", "namespace", hr.Namespace, "name", hr.Name)...)
		return nil
	}
	r.Log.V(1).Info("HelmRelease associated with layer",
		append(logging.GetFunctionAndSource(logging.MyCaller), "kind", "helmreleases.helm.toolkit.fluxcd.io", "namespace", hr.Namespace, "name", hr.Name, "layer", owner.Name)...)

	return []string{owner.Name}
}

func (r *AddonsLayerReconciler) indexHelmRepoByOwner(o runtime.Object) []string {
	logging.TraceCall(r.Log)
	defer logging.TraceExit(r.Log)

	r.Log.V(2).Info("indexing", logging.GetObjKindNamespaceName(o)...)
	hr, ok := o.(*sourcev1.HelmRepository)
	if !ok {
		r.Log.Error(fmt.Errorf("failed to cast to helmrepository"), "unable cast object to expected kind",
			logging.GetObjKindNamespaceName(o)...)
		return nil
	}
	owner := metav1.GetControllerOf(hr)
	if owner == nil {
		r.Log.V(2).Info("unexpected helmrepository passed to indexer, no owner information",
			append(logging.GetFunctionAndSource(logging.MyCaller), "kind", "helmrepositories.source.toolkit.fluxcd.io", "namespace", hr.Namespace, "name", hr.Name)...)
		return nil
	}
	if owner.APIVersion != kraanv1alpha1.GroupVersion.String() || owner.Kind != "AddonsLayer" {
		r.Log.V(2).Info("unexpected helmrepository passed to indexer, not owned by an AddonsLayer",
			append(logging.GetFunctionAndSource(logging.MyCaller), "kind", "helmrepositories.source.toolkit.fluxcd.io", "namespace", hr.Namespace, "name", hr.Name)...)
		return nil
	}
	r.Log.V(1).Info("Helm Repository associated with layer",
		append(logging.GetFunctionAndSource(logging.MyCaller), "kind", "helmrepositories.source.toolkit.fluxcd.io", "namespace", hr.Namespace, "name", hr.Name, "layer", owner.Name)...)
	return []string{owner.Name}
}
