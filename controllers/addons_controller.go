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
	"time"

	helmctlv2 "github.com/fluxcd/helm-controller/api/v2beta1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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
	"github.com/fidelity/kraan/pkg/layers"
	"github.com/fidelity/kraan/pkg/logging"
	"github.com/fidelity/kraan/pkg/metrics"
	"github.com/fidelity/kraan/pkg/repos"
)

var (
	hrOwnerKey = ".owner"
	reconciler *AddonsLayerReconciler
)

type AddonsLayerReconcilerOptions struct {
	MaxConcurrentReconciles int
}

func (r *AddonsLayerReconciler) SetupWithManagerAndOptions(mgr ctrl.Manager, opts AddonsLayerReconcilerOptions) error { // nolint: funlen,gocyclo // ok
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
		//Watch(repoKind, repoHandler).
		Owns(hr).
		Owns(hrepo).
		WithOptions(controller.Options{MaxConcurrentReconciles: opts.MaxConcurrentReconciles}).
		WithEventFilter(predicates(r.Log)).
		Build(r)
	if err != nil {
		return errors.Wrap(err, "error creating controller")
	}
	return ctl.Watch(
		&source.Kind{Type: &sourcev1.GitRepository{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(r.repoMapperFunc),
		},
		predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				r.Log.V(3).Info("create event for GitRepository",
					append(logging.GetFunctionAndSource(logging.MyCaller), "kind", logging.GitRepoSourceKind(), "data", logging.LogJSON(e))...)
				return true
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				r.Log.V(3).Info("update event for GitRepository",
					append(logging.GetFunctionAndSource(logging.MyCaller), "kind", logging.GitRepoSourceKind(), "data", logging.LogJSON(e))...)
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
					r.Log.V(1).Info("old revision", logging.GetGitRepoInfo(newRepo)...)
					return true
				}
				r.Log.V(1).Info("no change to revision, processing anyway",
					append(logging.GetFunctionAndSource(logging.MyCaller), logging.GetGitRepoInfo(newRepo)...)...)
				return true
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				r.Log.V(3).Info("delete event for GitRepository",
					append(logging.GetFunctionAndSource(logging.MyCaller), "kind", logging.GitRepoSourceKind(), "data", logging.LogJSON(e))...)
				return true
			},
			GenericFunc: func(e event.GenericEvent) bool {
				r.Log.V(3).Info("generic event for GitRepository",
					append(logging.GetFunctionAndSource(logging.MyCaller), "kind", logging.GitRepoSourceKind(), "data", logging.LogJSON(e))...)
				return true
			},
		},
	)
}

func predicates(logger logr.Logger) predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			logger.V(3).Info("create event", append(logging.GetFunctionAndSource(logging.MyCaller), "data", logging.LogJSON(e))...)
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			logger.V(3).Info("update event", append(logging.GetFunctionAndSource(logging.MyCaller), "data", logging.LogJSON(e))...)
			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			logger.V(3).Info("generic event", append(logging.GetFunctionAndSource(logging.MyCaller), "data", logging.LogJSON(e))...)
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
		return nil, errors.WithMessage(err, "failed to create reconciler")
	}
	reconciler.Context = context.Background()
	reconciler.Applier, err = apply.NewApplier(client, logger.WithName("applier"), scheme)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to create applier")
	}
	reconciler.Repos = repos.NewRepos(reconciler.Context, reconciler.Log)

	reconciler.Metrics = metrics.NewMetrics()
	return reconciler, err
}

func (r *AddonsLayerReconciler) getK8sClient() (kubernetes.Interface, error) {
	logging.TraceCall(r.Log)
	defer logging.TraceExit(r.Log)

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(r.Config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create k8s client")
	}

	return clientset, nil
}

func (r *AddonsLayerReconciler) processPrune(l layers.Layer) (statusReconciled bool, err error) {
	logging.TraceCall(r.Log)
	defer logging.TraceExit(r.Log)

	ctx := r.Context
	applier := r.Applier

	pruneIsRequired, hrs, err := applier.PruneIsRequired(ctx, l)
	if err != nil {
		return false, errors.WithMessage(err, "check for apply required failed")
	}
	if pruneIsRequired {
		l.SetStatusPruning()
		if pruneErr := applier.Prune(ctx, l, hrs); pruneErr != nil {
			return true, errors.WithMessage(pruneErr, "prune failed")
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

	applyIsRequired, err := applier.ApplyIsRequired(ctx, l)
	if err != nil {
		return false, errors.WithMessage(err, "check if apply is required failed")
	}
	if applyIsRequired {
		if !l.DependenciesDeployed() {
			l.SetDelayedRequeue()
			return true, nil
		}

		l.SetStatusApplying()
		if applyErr := applier.Apply(ctx, l); applyErr != nil {
			return true, errors.WithMessage(applyErr, "apply failed")
		}
		l.SetDelayedRequeue()
		return true, nil
	}
	return false, nil
}

func (r *AddonsLayerReconciler) checkSuccess(l layers.Layer) (string, error) {
	logging.TraceCall(r.Log)
	defer logging.TraceExit(r.Log)

	ctx := r.Context
	applier := r.Applier

	applyWasSuccessful, err := applier.ApplyWasSuccessful(ctx, l)
	if err != nil {
		return "", errors.WithMessage(err, "check for apply required failed")
	}
	if !applyWasSuccessful {
		l.SetDelayedRequeue()
		return "", nil
	}
	l.SetStatusDeployed()
	revision, err := r.getRevision(l)
	if err != nil {
		return "", errors.WithMessage(err, "failed to get revision")
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
					"namespace", l.GetSpec().Source.NameSpace, "name", l.GetSpec().Source.Name, "layer", l.GetName())...)
			return nil
		}
		r.Log.V(1).Info("waiting for layer data to be synced",
			append(logging.GetFunctionAndSource(logging.MyCaller), "layer", l.GetName(), "kind", logging.GitRepoSourceKind(),
				"namespace", l.GetSpec().Source.NameSpace, "name", l.GetSpec().Source.Name, "path", l.GetSpec().Source.Path)...)
		time.Sleep(time.Second)
	}
	l.StatusUpdate(kraanv1alpha1.FailedCondition, kraanv1alpha1.AddonsLayerFailedReason, err.Error())
	return errors.WithMessage(err, "failed to link to layer data")
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
				return false, errors.WithMessage(err, "failed to wait for layer data")
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
		reason := fmt.Sprintf("layer source: %s not ready.", repo.GetGitRepo().Name)
		message := fmt.Sprintf("source state: %s.", srcMsg)
		l.StatusUpdate(kraanv1alpha1.PendingCondition, reason, message)
		return false, nil
	}
	return true, nil
}

func (r *AddonsLayerReconciler) processAddonLayer(l layers.Layer) (string, error) { // nolint: gocyclo // ok
	logging.TraceCall(r.Log)
	defer logging.TraceExit(r.Log)

	l.GetLogger().Info("processing")

	if l.IsHold() {
		l.SetHold()
		return "", nil
	}

	if !l.CheckK8sVersion() {
		l.SetStatusK8sVersion()
		l.SetDelayedRequeue()
		return "", nil
	}

	ready, err := r.isReady(l)
	if err != nil {
		return "", errors.WithMessage(err, "failed to check source is ready")
	}
	if !ready {
		return "", nil
	}

	layerDataReady, err := r.checkData(l)
	if err != nil {
		return "", errors.WithMessage(err, "failed to check layer data is ready")
	}
	if !layerDataReady {
		return "", nil
	}

	layerStatusUpdated, err := r.processPrune(l)
	if err != nil {
		return "", errors.WithMessage(err, "failed to perform prune processing")
	}
	if layerStatusUpdated {
		return "", nil
	}

	layerStatusUpdated, err = r.processApply(l)
	if err != nil {
		return "", errors.WithMessage(err, "failed to perform apply processing")
	}
	if layerStatusUpdated {
		return "", nil
	}

	return r.checkSuccess(l)
}

func (r *AddonsLayerReconciler) updateRequeue(l layers.Layer, res *ctrl.Result, rerr *error) {
	logging.TraceCall(r.Log)
	defer logging.TraceExit(r.Log)

	if l.IsUpdated() {
		*rerr = r.update(r.Context, r.Log, l.GetAddonsLayer())
	}
	if l.NeedsRequeue() {
		if l.IsDelayed() {
			*res = ctrl.Result{Requeue: true, RequeueAfter: l.GetDelay()}
			return
		}
		*res = ctrl.Result{Requeue: true}
		return
	}
}

// Reconcile process AddonsLayers custom resources.
// +kubebuilder:rbac:groups=kraan.io,resources=addons,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kraan.io,resources=addons/status,verbs=get;update;patch
func (r *AddonsLayerReconciler) Reconcile(req ctrl.Request) (res ctrl.Result, err error) {
	logging.TraceCall(r.Log)
	defer logging.TraceExit(r.Log)

	ctx := r.Context
	reconcileStart := time.Now()

	var addonsLayer *kraanv1alpha1.AddonsLayer = &kraanv1alpha1.AddonsLayer{}
	if err = r.Get(ctx, req.NamespacedName, addonsLayer); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log := r.Log.WithValues("layer", req.NamespacedName.Name)

	l := layers.CreateLayer(ctx, r.Client, r.k8client, log, addonsLayer)
	deployedRevision, err := r.processAddonLayer(l)
	if err != nil {
		l.StatusUpdate(kraanv1alpha1.FailedCondition, kraanv1alpha1.AddonsLayerFailedReason, errors.Cause(err).Error())
		log.Error(err, "failed to process addons layer", logging.GetFunctionAndSource(logging.MyCaller)...)
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

	r.updateRequeue(l, &res, &err)
	return res, err
}

func (r *AddonsLayerReconciler) recordReadiness(al *kraanv1alpha1.AddonsLayer, deleted bool) {
	logging.TraceCall(r.Log)
	defer logging.TraceExit(r.Log)

	status := corev1.ConditionUnknown
	if al.Status.State == kraanv1alpha1.DeployedCondition {
		status = corev1.ConditionTrue
	}
	r.Metrics.RecordCondition(al, kraanv1alpha1.Condition{
		Type:   kraanv1alpha1.DeployedCondition,
		Status: status,
	}, deleted)
}

func (r *AddonsLayerReconciler) update(ctx context.Context, log logr.Logger,
	a *kraanv1alpha1.AddonsLayer) error {
	logging.TraceCall(r.Log)
	defer logging.TraceExit(r.Log)

	if err := r.Status().Update(ctx, a); err != nil {
		log.Error(err, "unable to update AddonsLayer status", logging.GetFunctionAndSource(logging.MyCaller)...)
		return err
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
		layer := layers.CreateLayer(r.Context, r.Client, r.k8client, r.Log, &addon) //nolint:scopelint // ok
		if layer.GetSpec().Source.Name == srcRepo.Name && layer.GetSpec().Source.NameSpace == srcRepo.Namespace {
			r.Log.V(1).Info("layer is using this source", append(logging.GetGitRepoInfo(srcRepo), append(logging.GetFunctionAndSource(logging.MyCaller), "layers", addons)...)...)
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
	}
	r.Log.Info("synced source", append(logging.GetGitRepoInfo(srcRepo), append(logging.GetFunctionAndSource(logging.MyCaller), "layers", addons)...)...)
	if err := repo.TidyRepo(); err != nil {
		r.Log.Error(err, "unable to garbage collect repo revisions", append(logging.GetGitRepoInfo(srcRepo), logging.GetFunctionAndSource(logging.MyCaller)...)...)
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
		r.Log.Info("unexpected helmrelease passed to indexer, no owner information",
			append(logging.GetFunctionAndSource(logging.MyCaller), "kind", "helmreleases.helm.toolkit.fluxcd.io", "namespace", hr.Namespace, "name", hr.Name)...)
		return nil
	}
	if owner.APIVersion != kraanv1alpha1.GroupVersion.String() || owner.Kind != "AddonsLayer" {
		r.Log.Info("unexpected helmrelease passed to indexer, not owned by an AddonsLayer",
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
		r.Log.Info("unexpected helmrepository passed to indexer, no owner information",
			append(logging.GetFunctionAndSource(logging.MyCaller), "kind", "helmrepositories.source.toolkit.fluxcd.io", "namespace", hr.Namespace, "name", hr.Name)...)
		return nil
	}
	if owner.APIVersion != kraanv1alpha1.GroupVersion.String() || owner.Kind != "AddonsLayer" {
		r.Log.Info("unexpected helmrepository passed to indexer, not owned by an AddonsLayer",
			append(logging.GetFunctionAndSource(logging.MyCaller), "kind", "helmrepositories.source.toolkit.fluxcd.io", "namespace", hr.Namespace, "name", hr.Name)...)
		return nil
	}
	r.Log.V(1).Info("Helm Repository associated with layer",
		append(logging.GetFunctionAndSource(logging.MyCaller), "kind", "helmrepositories.source.toolkit.fluxcd.io", "namespace", hr.Namespace, "name", hr.Name, "layer", owner.Name)...)
	return []string{owner.Name}
}
