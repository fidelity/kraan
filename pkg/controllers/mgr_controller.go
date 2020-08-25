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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kraanv1alpha1 "github.com/fidelity/kraan/pkg/api/v1alpha1"
	manager "github.com/fidelity/kraan/pkg/internal/manager"
	utils "github.com/fidelity/kraan/pkg/internal/utils"
)

// LayerMgrReconciler reconciles a AddonsConfig object
type LayerMgrReconciler struct {
	client.Client
	Log     logr.Logger
	Scheme  *runtime.Scheme
	Context context.Context
}

// NewLayerMgrReconciler returns an AddonsLayerReconciler instance
func NewLayerMgrReconciler(client client.Client, logger logr.Logger,
	scheme *runtime.Scheme) (reconciler *LayerMgrReconciler, err error) {
	reconciler = &LayerMgrReconciler{
		Client: client,
		Log:    logger,
		Scheme: scheme,
	}
	reconciler.Context = context.Background()
	return reconciler, err
}

func (r *LayerMgrReconciler) processLayerMgr(m manager.Manager) error {
	utils.Log(r.Log, 2, 1, "processing", "Status", m.GetStatus())

	if m.IsHold() {
		m.SetHold()
		return nil
	}

	return nil
}

func (r *LayerMgrReconciler) updateRequeue(m manager.Manager, res *ctrl.Result, rerr *error) {
	if m.IsUpdated() {
		*rerr = r.update(r.Context, r.Log, m.GetLayerMgr())
	}
	if m.NeedsRequeue() {
		if m.IsDelayed() {
			*res = ctrl.Result{RequeueAfter: m.GetDelay()}
			return
		}
		*res = ctrl.Result{Requeue: true}
		return
	}
}

// Reconcile process AddonsLayers custom resources.
// +kubebuilder:rbac:groups=kraan.io,resources=addons,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kraan.io,resources=addons/status,verbs=get;update;patch
func (r *LayerMgrReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := r.Context

	var layerMgr *kraanv1alpha1.LayerMgr = &kraanv1alpha1.LayerMgr{}
	if err := r.Get(ctx, req.NamespacedName, layerMgr); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log := r.Log.WithValues("requestName", req.NamespacedName.Name)

	m := manager.CreateManager(ctx, r.Client, log, layerMgr)
	var rerr error = nil
	var res ctrl.Result = ctrl.Result{}
	defer r.updateRequeue(m, &res, &rerr)
	err := r.processLayerMgr(m)
	if err != nil {
		m.StatusUpdate(kraanv1alpha1.FailedCondition, kraanv1alpha1.LayerMgrFailedReason, err.Error())
	}
	return res, rerr
}

func (r *LayerMgrReconciler) update(ctx context.Context, log logr.Logger,
	m *kraanv1alpha1.LayerMgr) error {
	if err := r.Status().Update(ctx, m); err != nil {
		log.Error(err, "unable to update LayerMgr status")
		return err
	}

	return nil
}

func (r *LayerMgrReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kraanv1alpha1.LayerMgr{}).
		Complete(r)
}
