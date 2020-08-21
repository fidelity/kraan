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
<<<<<<< HEAD
=======
	"time"
>>>>>>> a821dfc3b5990ac82c0e6fdc26e03b6b08afc0d6

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kraanv1alpha1 "github.com/fidelity/kraan/pkg/api/v1alpha1"
	"github.com/fidelity/kraan/pkg/internal/apply"
	layers "github.com/fidelity/kraan/pkg/internal/layers"
	utils "github.com/fidelity/kraan/pkg/internal/utils"
)

// AddonsLayerReconciler reconciles a AddonsLayer object.
type AddonsLayerReconciler struct {
	client.Client
	Log     logr.Logger
	Scheme  *runtime.Scheme
	Applier apply.LayerApplier
}

// NewReconciler returns an AddonsLayerReconciler instance
func NewReconciler(client client.Client, logger logr.Logger,
	scheme *runtime.Scheme) (reconciler *AddonsLayerReconciler, err error) {
	reconciler = &AddonsLayerReconciler{
		Client: client,
		Log:    logger,
		Scheme: scheme,
	}
	reconciler.Applier, err = apply.NewApplier(logger)
	return reconciler, err
}

func processFailed(l layers.Layer) {
	if l.GetStatus() == kraanv1alpha1.FailedCondition {
		return
		// Reset failed status if it was set more than 'interval' duration ago
		// Use previous condition to detect what to set status to
	}
}

func processAddonLayer(l layers.Layer) error { // nolint:gocyclo // ok
	utils.Log(l.GetLogger(), 2, 1, "processing", "Status", l.GetStatus())

	if l.IsHold() {
		l.SetHold()
		return nil
	}

	processFailed(l)

	if !l.IsVersionCurrent() {
		l.SetStatusPrunePending()
		return nil
	}

	if !l.CheckK8sVersion() {
		l.SetStatusPrunePending()
		l.SetDelayed()
		return nil
	}

	if l.IsPruningRequired() {
		l.SetStatusPruning()
		if err := l.Prune(); err != nil {
			return err
		}
		if err := l.SetAllPrunePending(); err != nil {
			return err
		}
		l.SetDelayed()
		return nil
	}

	l.SetStatusPruningToPruned()

	if l.AllPruned() {
		if err := l.SetAllPrunedToApplyPending(); err != nil {
			return err
		}
		return nil
	}

	if !l.DependenciesDeployed() {
		l.SetStatusApplyPending()
		l.SetDelayed()
		return nil
	}

<<<<<<< HEAD
	if l.IsApplyRequired() {
		l.SetStatusApplying()
		if err := l.Apply(); err != nil {
			return err
		}
		l.SetDelayed()
		return nil
	}

	l.SetStatusDeployed()
	return nil
=======
func processFailed(l *layers.Layer) {
	utils.Log(l.GetLogger(), 2, 1, "processing", "Status", kraanv1alpha1.FailedCondition)
	// Perform a retry if failed condition is more than 'interval' duration ago
	// Use previous condition to detect what to retry
	// However verify dependencies again in case something has changed in other AddonsLayers
	// so effectively reset status to PrunePending or ApplyPending depending on what failed.
	utils.Log(l.GetLogger(), 2, 1, "processed",
		"Previous Status", kraanv1alpha1.FailedCondition,
		"Status", l.GetFullStatus(),
		"Spec:", l.GetSpec())
>>>>>>> a821dfc3b5990ac82c0e6fdc26e03b6b08afc0d6
}

func processAddonLayer(l *layers.Layer) error {
	if l.IsHold() {
		l.SetHold()
		return nil
	}

	if !l.IsVersionCurrent() {
		// Version has changed set to prune pending.
		l.StatusPrunePending()
		l.SetRequeue()
		return
	}

	if !l.CheckK8sVersion() {
		l.StatusK8sVersionMismatch()
		return nil
	}

	if l.IsPruneRequired() {
		l.StatusPruning()
		l.SetAllPruePending()
		l.Prune()
		l.SetRequeue()
		return	
	}	

	// check if anyone else is pruning
		// set pruned
		// return
	
	// set anyone else who is purned to applypending

	// if dependsOn are not deployed
		// set applypending
		// return

	// IfApplyRequired
		// apply and set status to applying
		// return

	// set status to deployed

}

// Reconcile process AddonsLayers custom resources.
// +kubebuilder:rbac:groups=kraan.io,resources=addons,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kraan.io,resources=addons/status,verbs=get;update;patch
<<<<<<< HEAD
func (r *AddonsLayerReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
=======
func (r *AddonsLayerReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) { // nolint:gocyclo // ok
>>>>>>> a821dfc3b5990ac82c0e6fdc26e03b6b08afc0d6
	ctx := context.Background()

	var addonsLayer *kraanv1alpha1.AddonsLayer = &kraanv1alpha1.AddonsLayer{}
	if err := r.Get(ctx, req.NamespacedName, addonsLayer); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log := r.Log.WithValues(
		"requestNamespace", req.NamespacedName.Namespace,
		"requestName", req.NamespacedName.Name)

	l := layers.CreateLayer(ctx, r.Client, log, addonsLayer)

<<<<<<< HEAD
	err := processAddonLayer(l)
	if err != nil {
		l.StatusUpdate(kraanv1alpha1.FailedCondition, kraanv1alpha1.AddonsLayerFailedReason, err.Error())
=======
	
		processPrune(l, r)
		processPruning(l, r)
		processApplyPending(l)
		processApply(l, r)
		processApplying(l, r)
		processHold(l)
		processFailed(l)
	default:
		logBadStatusError(log, s)
>>>>>>> a821dfc3b5990ac82c0e6fdc26e03b6b08afc0d6
	}

	if l.IsUpdated() {
		if e := r.update(ctx, log, addonsLayer); e != nil {
			return ctrl.Result{Requeue: true}, e
		}
	}
	if l.NeedsRequeue() {
		if l.IsDelayed() {
			return ctrl.Result{RequeueAfter: l.GetDelay()}, err
		}
		return ctrl.Result{Requeue: true}, err
	}
	return ctrl.Result{}, err
}

func (r *AddonsLayerReconciler) update(ctx context.Context, log logr.Logger,
	a *kraanv1alpha1.AddonsLayer) error {
	if err := r.Status().Update(ctx, a); err != nil {
		log.Error(err, "unable to update AddonsLayer status")
		return err
	}

	return nil
}

/*
func (r *AddonsLayerReconciler) gitRepositorySource(o handler.MapObject) []ctrl.Request {
	//ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	//defer cancel()

	return []ctrl.Request{
		{
			NamespacedName: types.NamespacedName{
				//Name:      sourcev1.GitRepository.Name,
				//Namespace: sourcev1.GitRepository.Namespace,
			},
		},
	}
}
*/

// SetupWithManager is used to setup the controller
func (r *AddonsLayerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	addonsLayer := &kraanv1alpha1.AddonsLayer{}
	_, err := ctrl.NewControllerManagedBy(mgr).
		For(addonsLayer).
		/*		Watches(
				&source.Kind{Type: &sourcev1.GitRepository{}},
				&handler.EnqueueRequestsFromMapFunc{
					ToRequests: handler.ToRequestsFunc(r.gitRepositorySource),
				},
			).*/
		Build(r)

	if err != nil {
		return fmt.Errorf("failed setting up the AddonsLayer controller manager: %w", err)
	}
	/*
		if err = c.Watch(
			&source.Kind{Type: &*sourcev1.GitRepository{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: controllers.ClusterToInfrastructureMapFunc(awsManagedControlPlane.GroupVersionKind()),
			}); err != nil {
			return fmt.Errorf("failed adding a watch for ready clusters: %w", err)
		}
	*/

	return nil
}
