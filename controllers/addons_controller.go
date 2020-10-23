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

func (r *AddonsLayerReconciler) SetupWithManagerAndOptions(mgr ctrl.Manager, opts AddonsLayerReconcilerOptions) error {
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
		predicate.Funcs{CreateFunc: func(e event.CreateEvent) bool {
			r.Log.V(2).Info("create event for GitRepository", "kind", "gitrepositories.source.toolkit.fluxcd.io", "data", apply.LogJSON(e))
			return true
		},
			UpdateFunc: func(e event.UpdateEvent) bool {
				r.Log.V(2).Info("update event for GitRepository", "kind", "gitrepositories.source.toolkit.fluxcd.io", "data", apply.LogJSON(e))
				return true
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				r.Log.V(2).Info("delete event for GitRepository", "kind", "gitrepositories.source.toolkit.fluxcd.io", "data", apply.LogJSON(e))
				return true
			},
			GenericFunc: func(e event.GenericEvent) bool {
				r.Log.V(2).Info("generic event for GitRepository", "kind", "gitrepositories.source.toolkit.fluxcd.io", "data", apply.LogJSON(e))
				return true
			},
		},
	)
}

func predicates(logger logr.Logger) predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			logger.V(2).Info("create event", "data", apply.LogJSON(e))
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			logger.V(2).Info("update event", "data", apply.LogJSON(e))
			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			logger.V(2).Info("generic event", "data", apply.LogJSON(e))
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
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(r.Config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create k8s client")
	}

	return clientset, nil
}

func (r *AddonsLayerReconciler) processPrune(l layers.Layer) (statusReconciled bool, err error) {
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
			return true, errors.WithMessage(applyErr, "check for apply failed")
		}
		l.SetDelayedRequeue()
		return true, nil
	}
	return false, nil
}

func (r *AddonsLayerReconciler) checkSuccess(l layers.Layer) error {
	ctx := r.Context
	applier := r.Applier

	applyWasSuccessful, err := applier.ApplyWasSuccessful(ctx, l)
	if err != nil {
		return errors.WithMessage(err, "check for apply required failed")
	}
	if !applyWasSuccessful {
		l.SetDelayedRequeue()
		return nil
	}
	l.SetStatusDeployed()
	return nil
}

func (r *AddonsLayerReconciler) waitForData(l layers.Layer, repo repos.Repo) (err error) {
	MaxTries := 15
	for try := 1; try < MaxTries; try++ {
		err = repo.LinkData(l.GetSourcePath(), l.GetSpec().Source.Path)
		if err == nil {
			r.Log.V(1).Info("linked to layer data", "requestName", l.GetName(), "kind", "gitrepositories.source.toolkit.fluxcd.io",
				"namespace", l.GetSpec().Source.NameSpace, "name", l.GetSpec().Source.Name, "layer", l.GetName())
			return nil
		}
		r.Log.V(1).Info("waiting for layer data to be synced", "layer", l.GetName(),
			"kind", "gitrepositories.source.toolkit.fluxcd.io", "namespace", l.GetSpec().Source.NameSpace, "name", l.GetSpec().Source.Name,
			"path", l.GetSpec().Source.Path)
		time.Sleep(time.Second)
	}
	l.StatusUpdate(kraanv1alpha1.FailedCondition, kraanv1alpha1.AddonsLayerFailedReason, err.Error())
	return errors.WithMessage(err, "failed to link to layer data")
}

func (r *AddonsLayerReconciler) checkData(l layers.Layer) (bool, error) {
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
		r.Log.Info("waiting for layer data", "requestName", l.GetName(), "kind", "gitrepositories.source.toolkit.fluxcd.io", "source", l.GetSpec().Source)
		time.Sleep(time.Duration(time.Second * time.Duration(try))) // nolint: unconvert // ignore
	}
	l.SetDelayedRequeue()
	l.SetStatusPending()
	return false, nil
}

func (r *AddonsLayerReconciler) processAddonLayer(l layers.Layer) error {
	l.GetLogger().Info("processing")

	if l.IsHold() {
		l.SetHold()
		return nil
	}

	if !l.CheckK8sVersion() {
		l.SetStatusK8sVersion()
		l.SetDelayedRequeue()
		return nil
	}

	layerDataReady, err := r.checkData(l)
	if err != nil {
		return errors.WithMessage(err, "failed to check layer data is ready")
	}
	if !layerDataReady {
		return nil
	}

	layerStatusUpdated, err := r.processPrune(l)
	if err != nil {
		return errors.WithMessage(err, "failed to perform prune processing")
	}
	if layerStatusUpdated {
		return nil
	}

	layerStatusUpdated, err = r.processApply(l)
	if err != nil {
		return errors.WithMessage(err, "failed to perform apply processing")
	}
	if layerStatusUpdated {
		return nil
	}

	return r.checkSuccess(l)
}

func (r *AddonsLayerReconciler) updateRequeue(l layers.Layer, res *ctrl.Result, rerr *error) {
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
	ctx := r.Context
	reconcileStart := time.Now()

	var addonsLayer *kraanv1alpha1.AddonsLayer = &kraanv1alpha1.AddonsLayer{}
	if err = r.Get(ctx, req.NamespacedName, addonsLayer); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log := r.Log.WithValues("layer", req.NamespacedName.Name)

	l := layers.CreateLayer(ctx, r.Client, r.k8client, log, addonsLayer)
	err = r.processAddonLayer(l)
	if err != nil {
		l.StatusUpdate(kraanv1alpha1.FailedCondition, kraanv1alpha1.AddonsLayerFailedReason, err.Error())
		log.Error(err, "failed to process addons layer")
	}
	r.Metrics.RecordDuration(l.GetAddonsLayer(), reconcileStart)
	r.recordReadiness(l.GetAddonsLayer(), false)
	r.updateRequeue(l, &res, &err)
	return res, err
}

func (r *AddonsLayerReconciler) recordReadiness(al *kraanv1alpha1.AddonsLayer, deleted bool) {
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
	if err := r.Status().Update(ctx, a); err != nil {
		log.Error(err, "unable to update AddonsLayer status")
		return err
	}

	return nil
}

func (r *AddonsLayerReconciler) repoMapperFunc(a handler.MapObject) []reconcile.Request { // nolint: funlen,gocyclo // ok
	/* Not sure why this test fails when it shouldn't
	kind := a.Object.GetObjectKind().GroupVersionKind()
	repoKind := sourcev1.GitRepositoryKind
	if kind.Kind != repoKind {
		// If this isn't a GitRepository object, return an empty list of requests
		r.Log.Error(fmt.Errorf("unexpected object kind: %s, only %s supported", kind, sourcev1.GitRepositoryKind),
			"unexpected kind, continuing", "kind", "gitrepositories.source.toolkit.fluxcd.io", "data", apply.LogJSON(kind))
		//return []reconcile.Request{}
	}
	*/
	srcRepo, ok := a.Object.(*sourcev1.GitRepository)
	if !ok {
		r.Log.Error(fmt.Errorf("unable to cast object to GitRepository"), "skipping processing", "kind", "gitrepositories.source.toolkit.fluxcd.io", "data", apply.LogJSON(srcRepo))
		return []reconcile.Request{}
	}
	r.Log.V(1).Info("monitoring", "kind", "gitrepositories.source.toolkit.fluxcd.io", "namespace", srcRepo.Namespace, "name", srcRepo.Name, "data", apply.LogJSON(srcRepo))
	addonsList := &kraanv1alpha1.AddonsLayerList{}
	if err := r.List(r.Context, addonsList); err != nil {
		r.Log.Error(err, "unable to list AddonsLayers", "kind", "gitrepositories.source.toolkit.fluxcd.io",
			"namespace", srcRepo.Namespace, "name", srcRepo.Name, "data", apply.LogJSON(srcRepo))
		return []reconcile.Request{}
	}
	layerList := []layers.Layer{}
	addons := []reconcile.Request{}
	for _, addon := range addonsList.Items {
		layer := layers.CreateLayer(r.Context, r.Client, r.k8client, r.Log, &addon) //nolint:scopelint // ok
		if layer.GetSpec().Source.Name == srcRepo.Name && layer.GetSpec().Source.NameSpace == srcRepo.Namespace {
			r.Log.V(1).Info("layer is using this source", "kind", "gitrepositories.source.toolkit.fluxcd.io", "namespace", srcRepo.Namespace, "name", srcRepo.Name, "layer", addon.Name)
			layerList = append(layerList, layer)
			addons = append(addons, reconcile.Request{NamespacedName: types.NamespacedName{Name: layer.GetName(), Namespace: ""}})
		}
	}
	if len(addons) == 0 {
		return addons
	}
	repo := r.Repos.Add(srcRepo)
	r.Log.V(1).Info("created repo object", "kind", "gitrepositories.source.toolkit.fluxcd.io", "namespace", srcRepo.Namespace, "name", srcRepo.Namespace)
	if err := repo.SyncRepo(); err != nil {
		r.Log.Error(err, "unable to sync repo, not requeuing", "kind", "gitrepositories.source.toolkit.fluxcd.io", "namespace", srcRepo.Namespace, "name", srcRepo.Name)
		return []reconcile.Request{}
	}
	r.Log.V(1).Info("synced repo", "kind", "gitrepositories.source.toolkit.fluxcd.io", "namespace", srcRepo.Namespace, "name", srcRepo.Name)

	for _, layer := range layerList {
		if err := repo.LinkData(layer.GetSourcePath(), layer.GetSpec().Source.Path); err != nil {
			r.Log.Error(err, "unable to link referencing AddonsLayer directory to repository data",
				"kind", "gitrepositories.source.toolkit.fluxcd.io", "namespace", srcRepo.Namespace, "name", srcRepo.Name,
				"data", apply.LogJSON(srcRepo), "layer", layer.GetName())
			continue
		}
	}
	r.Log.Info("synced source", "kind", "gitrepositories.source.toolkit.fluxcd.io", "namespace", srcRepo.Namespace, "name", srcRepo.Name, "layers", addons)
	return addons
}

func (r *AddonsLayerReconciler) indexHelmReleaseByOwner(o runtime.Object) []string {
	r.Log.V(2).Info("indexing", "kind", "helmreleases.helm.toolkit.fluxcd.io", "data", apply.LogJSON(o))
	hr, ok := o.(*helmctlv2.HelmRelease)
	if !ok {
		r.Log.Error(fmt.Errorf("failed to cast to helmrelease"), "failed to cast object to expected kind",
			"kind", "helmreleases.helm.toolkit.fluxcd.io", "data", apply.LogJSON(o))
		return nil
	}
	owner := metav1.GetControllerOf(hr)
	if owner == nil {
		r.Log.Error(fmt.Errorf("no owner information"), "failed to process owner information",
			"kind", "helmreleases.helm.toolkit.fluxcd.io", "namespace", hr.Namespace, "name", hr.Name)
		return nil
	}
	if owner.APIVersion != kraanv1alpha1.GroupVersion.String() || owner.Kind != "AddonsLayer" {
		r.Log.Error(fmt.Errorf("helmrelease not owned by an AddonsLayer"), "unexpected helmrelease passed to indexer",
			"kind", "helmreleases.helm.toolkit.fluxcd.io", "namespace", hr.Namespace, "name", hr.Name)
		return nil
	}
	r.Log.V(1).Info("HelmRelease associated with layer", "kind", "helmreleases.helm.toolkit.fluxcd.io", "namespace", hr.Namespace, "name", hr.Name, "layer", owner.Name)

	return []string{owner.Name}
}

func (r *AddonsLayerReconciler) indexHelmRepoByOwner(o runtime.Object) []string {
	r.Log.V(2).Info("indexing", "kind", "helmrepositories.source.toolkit.fluxcd.io", "data", apply.LogJSON(o))
	hr, ok := o.(*sourcev1.HelmRepository)
	if !ok {
		r.Log.Error(fmt.Errorf("failed to cast to helmrepository"), "unable cast object to expected kind",
			"kind", "helmrepositories.helm.toolkit.fluxcd.io", "data", apply.LogJSON(o))
		return nil
	}
	owner := metav1.GetControllerOf(hr)
	if owner == nil {
		r.Log.Error(fmt.Errorf("no owner information"), "failed to process owner information",
			"kind", "helmrepositories.source.toolkit.fluxcd.io", "namespace", hr.Namespace, "name", hr.Name)
		return nil
	}
	if owner.APIVersion != kraanv1alpha1.GroupVersion.String() || owner.Kind != "AddonsLayer" {
		r.Log.Error(fmt.Errorf("helmrepository not owned by an AddonsLayer"), "unexpected helmrepository passed to indexer",
			"kind", "helmrepositories.source.toolkit.fluxcd.io", "namespace", hr.Namespace, "name", hr.Name)
		return nil
	}
	r.Log.V(1).Info("Helm Repository associated with layer", "kind", "helmrepositories.source.toolkit.fluxcd.io", "namespace", hr.Namespace, "name", hr.Name, "layer", owner.Name)
	return []string{owner.Name}
}
