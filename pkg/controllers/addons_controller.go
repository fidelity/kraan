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
	"os"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
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
	Context context.Context
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
	reconciler.Context = context.Background()
	reconciler.Applier, err = apply.NewApplier(client, logger, scheme)
	return reconciler, err
}

// GetK8sClient gets the Kubernetes client.
func GetK8sClient() (*kubernetes.Clientset, error) {
	kubeConfig := os.Getenv("KUBECONFIG")
	if len(kubeConfig) > 0 {
		// use the current context in kubeconfig
		config, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
		if err != nil {
			return nil, err
		}

		// create the clientset
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			return nil, err
		}

		return clientset, nil
	}
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	return clientset, nil
}

func logBadStatusError(log logr.Logger, status string) {
	utils.Log(log, 3, 1, fmt.Sprintf("invalid AddonLayer status: %s, ignoring", status))
}

func processEmptyStatus(l layers.Layer) {
	utils.Log(l.GetLogger(), 2, 1, "processing", "Status", "")
	l.StatusPrunePending()
	l.SetRequeue()
	utils.Log(l.GetLogger(), 2, 1, "processed",
		"Previous Status", "",
		"Status", l.GetFullStatus(),
		"Spec:", l.GetSpec())
}

func processDeployed(l layers.Layer) {
	utils.Log(l.GetLogger(), 2, 1, "processing", "Status", kraanv1alpha1.DeployedCondition)
	if !l.IsVersionCurrent() {
		// Version has changed set to prune pending.
		l.StatusPrunePending()
		l.SetRequeue()
	}
	utils.Log(l.GetLogger(), 2, 1, "processed",
		"Previous Status", kraanv1alpha1.DeployedCondition,
		"Status", l.GetFullStatus(),
		"Spec:", l.GetSpec())
}

func processPrunePending(l layers.Layer) {
	utils.Log(l.GetLogger(), 2, 1, "processing", "Status", kraanv1alpha1.PrunePendingCondition)
	if !l.CheckK8sVersion() {
		l.SetRequeue()
		l.SetDelayed()
		return
	}
	l.StatusPrune()
	// Thinking about this, implementing a reliable reverse DependsOn is non trivial so suggest
	// that for now we just transition to Prune state and revist waiting for layers that depend on
	// this layer later.
	utils.Log(l.GetLogger(), 2, 1, "processed",
		"Previous Status", kraanv1alpha1.PrunePendingCondition,
		"Status", l.GetFullStatus(),
		"Spec:", l.GetSpec())
}

func processPrune(l layers.Layer, r *AddonsLayerReconciler) {
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

func processPruning(l layers.Layer, r *AddonsLayerReconciler) {
	utils.Log(l.GetLogger(), 2, 1, "processing", "Status", kraanv1alpha1.PruningCondition)
	// Check if pruning is done, if not restart prune here in background with time limit of spec timeout value
	// Update status with details of pruning progress
	// If completed, set status to ApplyPending
}

func processApplyPending(l layers.Layer) {
	utils.Log(l.GetLogger(), 2, 1, "processing", "Status", kraanv1alpha1.ApplyPendingCondition)
	if !l.CheckK8sVersion() {
		l.SetRequeue()
		l.SetDelayed()
		return
	}
	// Wait for DependsOn layers to be Deployed
}

func processApply(l layers.Layer, r *AddonsLayerReconciler) {
	utils.Log(l.GetLogger(), 2, 1, "processing", "Status", kraanv1alpha1.ApplyCondition)
	// Start apply here in background thread with time limit of spec timeout value
	l.StatusApplying()
}

func processApplying(l layers.Layer, r *AddonsLayerReconciler) {
	utils.Log(l.GetLogger(), 2, 1, "processing", "Status", kraanv1alpha1.ApplyingCondition)
	// Check if applying is done, if not restart applyhere in background with time limit of spec timeout value
	// Update status with details of applying progress
	// If completed, set status to Deployed
}

func processHold(l layers.Layer) {
	utils.Log(l.GetLogger(), 2, 0, "processing", "Status", kraanv1alpha1.HoldCondition)
	if l.IsHold() {
		l.SetHold()
	}
	utils.Log(l.GetLogger(), 2, 1, "processed",
		"Previous Status", kraanv1alpha1.HoldCondition,
		"Status", l.GetFullStatus(),
		"Spec:", l.GetSpec())
}

func processFailed(l layers.Layer) {
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

// Reconcile process AddonsLayers custom resources.
// +kubebuilder:rbac:groups=kraan.io,resources=addons,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kraan.io,resources=addons/status,verbs=get;update;patch
func (r *AddonsLayerReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) { // nolint:gocyclo,funlen // ok
	ctx := r.Context

	var addonsLayer *kraanv1alpha1.AddonsLayer = &kraanv1alpha1.AddonsLayer{}
	if err := r.Get(ctx, req.NamespacedName, addonsLayer); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log := r.Log.WithValues(
		"requestNamespace", req.NamespacedName.Namespace,
		"requestName", req.NamespacedName.Name)

	k8sClient, err := GetK8sClient()
	if err != nil {
		log.Error(err, "unable to get kubernetes client")
	}
	l := layers.CreateLayer(ctx, k8sClient, log, addonsLayer)

	s := l.GetStatus()
	switch s {
	case "":
		processEmptyStatus(l)
	case kraanv1alpha1.DeployedCondition:
		processDeployed(l)
	case kraanv1alpha1.PrunePendingCondition:
		processPrunePending(l)

	case kraanv1alpha1.PruneCondition:
		processPrune(l, r)
	case kraanv1alpha1.PruningCondition:
		processPruning(l, r)
	case kraanv1alpha1.ApplyPendingCondition:
		processApplyPending(l)
	case kraanv1alpha1.ApplyCondition:
		processApply(l, r)
	case kraanv1alpha1.ApplyingCondition:
		processApplying(l, r)
	case kraanv1alpha1.HoldCondition:
		processHold(l)
	case kraanv1alpha1.FailedCondition:
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
