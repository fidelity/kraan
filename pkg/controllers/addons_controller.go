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

	helmopv1 "github.com/fluxcd/helm-operator/pkg/apis/helm.fluxcd.io/v1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1alpha1"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	kraanv1alpha1 "github.com/fidelity/kraan/pkg/api/v1alpha1"
	"github.com/fidelity/kraan/pkg/internal/apply"
	layers "github.com/fidelity/kraan/pkg/internal/layers"
	utils "github.com/fidelity/kraan/pkg/internal/utils"
)

var (
	hrOwnerKey = ".owner"
)

// AddonsLayerReconciler reconciles a AddonsLayer object.
type AddonsLayerReconciler struct {
	client.Client
	Config   *rest.Config
	k8client kubernetes.Interface
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Context  context.Context
	Applier  apply.LayerApplier
}

// NewReconciler returns an AddonsLayerReconciler instance
func NewReconciler(config *rest.Config, client client.Client, logger logr.Logger,
	scheme *runtime.Scheme) (reconciler *AddonsLayerReconciler, err error) {
	reconciler = &AddonsLayerReconciler{
		Config: config,
		Client: client,
		Log:    logger,
		Scheme: scheme,
	}
	reconciler.k8client = reconciler.getK8sClient()
	reconciler.Context = context.Background()
	reconciler.Applier, err = apply.NewApplier(client, logger, scheme, config)
	return reconciler, err
}

func (r *AddonsLayerReconciler) getK8sClient() kubernetes.Interface {
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(r.Config)
	if err != nil {
		//TODO - Adjust error handling?
		panic(err.Error())
	}

	return clientset
}

func (r *AddonsLayerReconciler) processPrune(l layers.Layer) (statusReconciled bool, err error) {
	ctx := r.Context
	applier := r.Applier

	pruneIsRequired, hrs, err := applier.PruneIsRequired(ctx, l)
	if err != nil {
		return false, err
	} else if pruneIsRequired {
		l.SetStatusPruning()
		if pruneErr := applier.Prune(ctx, l, hrs); err != nil {
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
		return false, err
	} else if applyIsRequired {
		if !l.DependenciesDeployed() {
			l.SetDelayedRequeue()
			return true, nil
		}

		l.SetStatusApplying()
		if applyErr := applier.Apply(ctx, l); err != nil {
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
		// TODO - we might want to add some sort of error handling here
		return err
	} else if applyWasSuccessful {
		l.SetStatusDeployed()
		return nil
	}
	return nil
}

func (r *AddonsLayerReconciler) processAddonLayer(l layers.Layer) error {
	utils.Log(r.Log, 1, 1, "processing", "Name", l.GetName(), "Status", l.GetStatus())

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

func repoMapperFunc(a handler.MapObject) []reconcile.Request {
	kind := a.Object.GetObjectKind().GroupVersionKind().Kind
	repoKind := sourcev1.GitRepositoryKind
	if kind != repoKind {
		// If this isn't a GitRepository object, return an empty list of requests
		// TODO - not sure if this is the correct way to handle this error
		return []reconcile.Request{}
	}
	repo, ok := a.Object.(*sourcev1.GitRepository)
	if !ok {
		return nil
	}
	namespace := repo.GetNamespace()
	name := repo.GetName()
	// TODO - here is where we need to map the GitRepository to the AddonsLayer(s)
	firstLayerName := name + "first-layer"
	secondLayerName := name + "second-layer"
	return []reconcile.Request{
		{NamespacedName: types.NamespacedName{
			Name:      firstLayerName,
			Namespace: namespace,
		}},
		{NamespacedName: types.NamespacedName{
			Name:      secondLayerName,
			Namespace: namespace,
		}},
	}
}

func indexHelmReleaseByOwner(o runtime.Object) []string {
	hr, ok := o.(*helmopv1.HelmRelease)
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
	return []string{owner.Name}
}

// SetupWithManager is used to setup the controller
func (r *AddonsLayerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	addonsLayer := &kraanv1alpha1.AddonsLayer{}
	hr := &helmopv1.HelmRelease{}

	if err := mgr.GetFieldIndexer().IndexField(r.Context, &helmopv1.HelmRelease{}, hrOwnerKey, indexHelmReleaseByOwner); err != nil {
		return fmt.Errorf("failed setting up FieldIndexer for HelmRelease owner: %w", err)
	}

	repoKind := &source.Kind{Type: &sourcev1.GitRepository{}}
	repoHandler := &handler.EnqueueRequestsFromMapFunc{ToRequests: handler.ToRequestsFunc(repoMapperFunc)}

	return ctrl.NewControllerManagedBy(mgr).
		For(addonsLayer).
		Owns(hr).
		Watches(repoKind, repoHandler).
		Complete(r)
}
