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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kraanv1alpha1 "github.com/fidelity/kraan/pkg/api/v1alpha1"
	"github.com/fidelity/kraan/pkg/internal/actors"
	layers "github.com/fidelity/kraan/pkg/internal/layers"
	utils "github.com/fidelity/kraan/pkg/internal/utils"
)

// AddonsLayerReconciler reconciles a AddonsLayer object.
type AddonsLayerReconciler struct {
	client.Client
	Log     logr.Logger
	Scheme  *runtime.Scheme
	Applier actors.LayerApplier
}

// NewReconciler returns an AddonsLayerReconciler instance
func NewReconciler(client client.Client, logger logr.Logger,
	scheme *runtime.Scheme) (reconciler *AddonsLayerReconciler, err error) {
	reconciler = &AddonsLayerReconciler{
		Client: client,
		Log:    logger,
		Scheme: scheme,
	}
	reconciler.Applier, err = actors.NewApplier(logger)
	return reconciler, err
}

func logBadStatusError(log logr.Logger, status string) {
	utils.Log(log, 3, 1, fmt.Sprintf("invalid AddonLayer status: %s, ignoring", status))
}

func processEmptyStatus(l *layers.Layer) {
	utils.Log(l.GetLogger(), 2, 1, "processing", "Status", "")
	
	l.SetRequeue()
	utils.Log(l.GetLogger(), 2, 1, "processed",
		"Previous Status", "",
		"Status", l.GetFullStatus(),
		"Spec:", l.GetSpec())
}

func processDeployed(l *layers.Layer) {
	utils.Log(l.GetLogger(), 2, 1, "processing", "Status", kraanv1alpha1.DeployedCondition)

	utils.Log(l.GetLogger(), 2, 1, "processed",
		"Previous Status", kraanv1alpha1.DeployedCondition,
		"Status", l.GetFullStatus(),
		"Spec:", l.GetSpec())
}

func processPrune(l *layers.Layer, r *AddonsLayerReconciler) {
	utils.Log(l.GetLogger(), 2, 1, "processing", "Status", kraanv1alpha1.PruneCondition)
	l.StatusPruning()
	if err := r.update(l.GetContext(), l.GetLogger(), l.GetAddonsLayer()); err != nil {
		l.SetRequeue()
		return
	}
	// Start prune here in background thread with time limit of spec timeout value
	// then wait for it to complete or timeout
	time.Sleep(time.Second * 60)
}

func processPruning(l *layers.Layer, r *AddonsLayerReconciler) {
	utils.Log(l.GetLogger(), 2, 1, "processing", "Status", kraanv1alpha1.PruningCondition)
	// Check if pruning is done, if not restart prune here in background with time limit of spec timeout value
	// Update status with details of pruning progress
	// If completed, set status to ApplyPending
}

func processApplyPending(l *layers.Layer) {
	utils.Log(l.GetLogger(), 2, 1, "processing", "Status", kraanv1alpha1.ApplyPendingCondition)
	if !l.CheckK8sVersion() {
		l.SetRequeue()
		l.SetDelayed()
		return
	}
	// Wait for DependsOn layers to be Deployed
}

func processApply(l *layers.Layer, r *AddonsLayerReconciler) {
	utils.Log(l.GetLogger(), 2, 1, "processing", "Status", kraanv1alpha1.ApplyCondition)
	// Start apply here in background thread with time limit of spec timeout value
	l.StatusApplying()
}

func processApplying(l *layers.Layer, r *AddonsLayerReconciler) {
	utils.Log(l.GetLogger(), 2, 1, "processing", "Status", kraanv1alpha1.ApplyingCondition)
	// Check if applying is done, if not restart applyhere in background with time limit of spec timeout value
	// Update status with details of applying progress
	// If completed, set status to Deployed
}

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
func (r *AddonsLayerReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) { // nolint:gocyclo // ok
	ctx := context.Background()

	var addonsLayer *kraanv1alpha1.AddonsLayer = &kraanv1alpha1.AddonsLayer{}
	if err := r.Get(ctx, req.NamespacedName, addonsLayer); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log := r.Log.WithValues(
		"requestNamespace", req.NamespacedName.Namespace,
		"requestName", req.NamespacedName.Name)

	l := layers.CreateLayer(ctx, r.Client, log, addonsLayer)

	
		processPrune(l, r)
		processPruning(l, r)
		processApplyPending(l)
		processApply(l, r)
		processApplying(l, r)
		processHold(l)
		processFailed(l)
	default:
		logBadStatusError(log, s)
	}

	if l.IsUpdated() {
		if err := r.update(ctx, log, addonsLayer); err != nil {
			return ctrl.Result{Requeue: true}, err
		}
	}
	if l.NeedsRequeue() {
		if l.IsDelayed() {
			return ctrl.Result{RequeueAfter: l.GetDelay()}, nil
		}
		return ctrl.Result{Requeue: true}, nil
	}
	return ctrl.Result{}, nil
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
