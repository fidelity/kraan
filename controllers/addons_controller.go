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

	helmctlv2 "github.com/fluxcd/helm-controller/api/v2alpha1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
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
	layers "github.com/fidelity/kraan/pkg/layers"
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

	if err := mgr.GetFieldIndexer().IndexField(r.Context, &helmctlv2.HelmRelease{}, hrOwnerKey, indexHelmReleaseByOwner); err != nil {
		return fmt.Errorf("failed setting up FieldIndexer for HelmRelease owner: %w", err)
	}

	if err := mgr.GetFieldIndexer().IndexField(r.Context, &sourcev1.HelmRepository{}, hrOwnerKey, indexHelmRepoByOwner); err != nil {
		return fmt.Errorf("failed setting up FieldIndexer for HelmRepository owner: %w", err)
	}

	ctl, err := ctrl.NewControllerManagedBy(mgr).
		For(addonsLayer).
		//Watch(repoKind, repoHandler).
		Owns(hr).
		Owns(hrepo).
		WithOptions(controller.Options{MaxConcurrentReconciles: opts.MaxConcurrentReconciles}).
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
			r.Log.V(3).Info("create event for GitRepository", "kind", "gitrepositories.source.toolkit.fluxcd.io", "data", apply.LogJSON(e))
			return true
		},
			UpdateFunc: func(e event.UpdateEvent) bool {
				r.Log.V(3).Info("update event for GitRepository", "kind", "gitrepositories.source.toolkit.fluxcd.io", "data", apply.LogJSON(e))
				return true
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				r.Log.V(3).Info("delete event for GitRepository", "kind", "gitrepositories.source.toolkit.fluxcd.io", "data", apply.LogJSON(e))
				return true
			},
			GenericFunc: func(e event.GenericEvent) bool {
				r.Log.V(3).Info("generic event for GitRepository", "kind", "gitrepositories.source.toolkit.fluxcd.io", "data", apply.LogJSON(e))
				return true
			},
		},
	)
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
		return nil, err
	}
	reconciler.Context = context.Background()
	reconciler.Applier, err = apply.NewApplier(client, logger.WithName("applier"), scheme)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create applier")
	}
	reconciler.Repos = repos.NewRepos(reconciler.Context, reconciler.Log)
	return reconciler, err
}

func (r *AddonsLayerReconciler) getK8sClient() (kubernetes.Interface, error) {
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(r.Config)
	if err != nil {
		return nil, err
	}

	return clientset, nil
}

func (r *AddonsLayerReconciler) processPrune(l layers.Layer) (statusReconciled bool, err error) {
	ctx := r.Context
	applier := r.Applier

	pruneIsRequired, hrs, err := applier.PruneIsRequired(ctx, l)
	if err != nil {
		r.Log.Error(err, "check for apply required failed", "requestName", l.GetName())
		return false, err
	} else if pruneIsRequired {
		l.SetStatusPruning()
		if pruneErr := applier.Prune(ctx, l, hrs); pruneErr != nil {
			r.Log.Error(pruneErr, "prune failed", "requestName", l.GetName())
			return true, pruneErr
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
		r.Log.Error(err, "check for apply required failed", "requestName", l.GetName())
		return false, err
	} else if applyIsRequired {
		r.Log.Info("apply required", "requestName", l.GetName(), "Spec", l.GetSpec(), "Status", l.GetFullStatus())
		if !l.DependenciesDeployed() {
			l.SetDelayedRequeue()
			return true, nil
		}

		l.SetStatusApplying()
		if applyErr := applier.Apply(ctx, l); applyErr != nil {
			r.Log.Error(applyErr, "check for apply failed", "requestName", l.GetName())
			return true, applyErr
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
		r.Log.Error(err, "check for apply required failed", "requestName", l.GetName())
		return err
	}
	if !applyWasSuccessful {
		l.SetDelayedRequeue()
		return nil
	}
	l.SetStatusDeployed()
	return nil
}

func (r *AddonsLayerReconciler) processAddonLayer(l layers.Layer) error {
	r.Log.Info("processing", "requestName", l.GetName(), "Status", l.GetStatus())

	if l.IsHold() {
		l.SetHold()
		return nil
	}

	if !l.CheckK8sVersion() {
		l.SetStatusK8sVersion()
		l.SetDelayedRequeue()
		return nil
	}

	layerStatusUpdated, err := r.processPrune(l)
	if err != nil {
		return err
	}
	if layerStatusUpdated {
		return nil
	}

	layerStatusUpdated, err = r.processApply(l)
	if err != nil {
		return err
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
func (r *AddonsLayerReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := r.Context

	var addonsLayer *kraanv1alpha1.AddonsLayer = &kraanv1alpha1.AddonsLayer{}
	if err := r.Get(ctx, req.NamespacedName, addonsLayer); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log := r.Log.WithValues("requestName", req.NamespacedName.Name)

	l := layers.CreateLayer(ctx, r.Client, r.k8client, log, addonsLayer)
	var rerr error = nil
	var res ctrl.Result = ctrl.Result{}
	//defer r.updateRequeue(l, &res, &rerr)
	err := r.processAddonLayer(l)
	if err != nil {
		l.StatusUpdate(kraanv1alpha1.FailedCondition, kraanv1alpha1.AddonsLayerFailedReason, err.Error())
	}
	r.updateRequeue(l, &res, &rerr)
	return res, rerr
}

func (r *AddonsLayerReconciler) update(ctx context.Context, log logr.Logger,
	a *kraanv1alpha1.AddonsLayer) error {
	if err := r.Status().Update(ctx, a); err != nil {
		log.Error(err, "unable to update AddonsLayer status")
		return err
	}

	return nil
}

func (r *AddonsLayerReconciler) repoMapperFunc(a handler.MapObject) []reconcile.Request {
	kind := a.Object.GetObjectKind().GroupVersionKind()
	repoKind := sourcev1.GitRepositoryKind
	if kind.Kind != repoKind {
		// If this isn't a GitRepository object, return an empty list of requests
		r.Log.Error(fmt.Errorf("unexpected object kind: %s, only %s supported", kind, sourcev1.GitRepositoryKind),
			"unexpected kind, continuing", "kind", "gitrepositories.source.toolkit.fluxcd.io", "data", apply.LogJSON(kind))
		//return []reconcile.Request{}
	}
	srcRepo, ok := a.Object.(*sourcev1.GitRepository)
	if !ok {
		r.Log.Error(fmt.Errorf("unable to cast object to GitRepository"), "skipping processing", "kind", "gitrepositories.source.toolkit.fluxcd.io", "data", apply.LogJSON(srcRepo))
		return []reconcile.Request{}
	}
	r.Log.V(1).Info("monitoring", "kind", "gitrepositories.source.toolkit.fluxcd.io", "data", apply.LogJSON(srcRepo))
	repo := r.Repos.Add(srcRepo)
	if err := repo.SyncRepo(); err != nil {
		r.Log.Error(err, "unable to sync repo, not requeuing", "kind", "gitrepositories.source.toolkit.fluxcd.io", "data", apply.LogJSON(srcRepo))
		return []reconcile.Request{}
	}
	addonsList := &kraanv1alpha1.AddonsLayerList{}
	if err := r.List(r.Context, addonsList); err != nil {
		r.Log.Error(err, "unable to list AddonsLayers", "kind", "gitrepositories.source.toolkit.fluxcd.io", "data", apply.LogJSON(srcRepo))
		return []reconcile.Request{}
	}
	addons := []reconcile.Request{}
	for _, addon := range addonsList.Items {
		layer := layers.CreateLayer(r.Context, r.Client, r.k8client, r.Log, &addon) //nolint:scopelint // ok
		if err := repo.LinkData(layer.GetSourcePath(), layer.GetSpec().Source.Path); err != nil {
			r.Log.Error(err, "unable to link AddonsLayer directory to repository data",
				"kind", "gitrepositories.source.toolkit.fluxcd.io", "data", apply.LogJSON(srcRepo), "layer", addon.Name)
			continue
		}

		r.Log.Info("adding layer to list", "kind", "gitrepositories.source.toolkit.fluxcd.io", "data", apply.LogJSON(srcRepo), "layer", addon.Name)
		if layer.GetSpec().Source.Name == repo.GetSourceName() && layer.GetSpec().Source.NameSpace == repo.GetSourceNameSpace() {
			addons = append(addons, reconcile.Request{NamespacedName: types.NamespacedName{Name: layer.GetName(), Namespace: ""}})
		}
	}
	return addons
}

func indexHelmReleaseByOwner(o runtime.Object) []string {
	log := ctrl.Log.WithName("HelmRelease sync")
	log.V(3).Info("indexing", "helmreleases.helm.toolkit.fluxcd.io", apply.LogJSON(o))
	hr, ok := o.(*helmctlv2.HelmRelease)
	if !ok {
		return nil
	}
	owner := metav1.GetControllerOf(hr)
	if owner == nil {
		return nil
	}
	if owner.APIVersion != kraanv1alpha1.GroupVersion.String() || owner.Kind != "AddonsLayer" {
		return nil
	}
	log.V(1).Info("HR associated with layer", "Layer Name", owner.Name, "HR", fmt.Sprintf("%s/%s", hr.GetNamespace(), hr.GetName()))

	return []string{owner.Name}
}

func indexHelmRepoByOwner(o runtime.Object) []string {
	log := ctrl.Log.WithName("HelmRepo sync")
	log.V(3).Info("indexing", "helmrepositories.source.toolkit.fluxcd.io", apply.LogJSON(o))
	hr, ok := o.(*sourcev1.HelmRepository)
	if !ok {
		return nil
	}
	owner := metav1.GetControllerOf(hr)
	if owner == nil {
		return nil
	}
	if owner.APIVersion != kraanv1alpha1.GroupVersion.String() || owner.Kind != "AddonsLayer" {
		return nil
	}
	log.V(1).Info("Helm Repository associated with layer", "Layer Name", owner.Name, "HR", fmt.Sprintf("%s/%s", hr.GetNamespace(), hr.GetName()))
	return []string{owner.Name}
}
