//Package manager provides functionality to processes ManagerMgr
//go:generate mockgen -destination=mockManagers.go -package=manager -source=manager.go . Manager
package manager

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kraanv1alpha1 "github.com/fidelity/kraan/pkg/api/v1alpha1"
)

// MaxConditions is the maximum number of condtions to retain.
var MaxConditions = 10

// Manager defines the interface for managing the layer manager.
type Manager interface {
	StatusUpdate(phase, reason, message string)
	setStatus(phase, reason, message string)

	IsHold() bool
	SetHold()
	SetPrune()
	SetDeploy()
	SetDone()
	GetStatus() string
	GetName() string
	GetLogger() logr.Logger
	GetContext() context.Context
	IsUpdated() bool
	NeedsRequeue() bool
	IsDelayed() bool
	GetDelay() time.Duration
	SetRequeue()
	SetDelayed()
	SetUpdated()
	GetFullStatus() *kraanv1alpha1.LayerMgrStatus
	GetSpec() *kraanv1alpha1.LayerMgrSpec
	GetLayerMgr() *kraanv1alpha1.LayerMgr
}

// KraanManager is the Schema for the addons API.
type KraanManager struct {
	updated  bool
	requeue  bool
	delayed  bool
	delay    time.Duration
	ctx      context.Context
	client   client.Client
	log      logr.Logger
	Manager  `json:"-"`
	LayerMgr *kraanv1alpha1.LayerMgr
}

// CreateManager creates a manager object.
func CreateManager(ctx context.Context, client client.Client,
	log logr.Logger, LayerMgr *kraanv1alpha1.LayerMgr) Manager {
	l := &KraanManager{
		requeue:  false,
		delayed:  false,
		updated:  false,
		ctx:      ctx,
		client:   client,
		log:      log,
		LayerMgr: LayerMgr,
	}
	l.delay = time.Second * 30
	return l
}

// SetRequeue sets the requeue flag to cause the LayerMgr to be requeued.
func (l *KraanManager) SetRequeue() {
	l.requeue = true
}

// SetUpdated sets the updated flag to cause the LayerMgr to update the custom resource.
func (l *KraanManager) SetUpdated() {
	l.updated = true
}

// SetDelayed sets the delayed flag to cause the LayerMgr to delay the requeue.
func (l *KraanManager) SetDelayed() {
	l.delayed = true
}

// GetFullStatus returns the LayerMgrs Status sub resource.
func (l *KraanManager) GetFullStatus() *kraanv1alpha1.LayerMgrStatus {
	return &l.LayerMgr.Status
}

// GetSpec returns the LayerMgrs Spec.
func (l *KraanManager) GetSpec() *kraanv1alpha1.LayerMgrSpec {
	return &l.LayerMgr.Spec
}

// GetLayerMgr returns the LayerMgrs Spec.
func (l *KraanManager) GetLayerMgr() *kraanv1alpha1.LayerMgr {
	return l.LayerMgr
}

// TODO - Paul do you still need this function?
/*
func (l *KraanManager) trimConditions() {
	for {
		if len(l.LayerMgr.Status.Conditions) > MaxConditions {
			t := l.LayerMgr.Status.Conditions[1:]
			l.LayerMgr.Status.Conditions = t // might losee order, will need a sort
		}
	}
}
*/

func (l *KraanManager) setStatus(phase, reason, message string) {
	l.LayerMgr.Status.Conditions = append(l.LayerMgr.Status.Conditions, kraanv1alpha1.MgrCondition{
		Type:               phase,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	})
	//l.trimConditions()
	l.LayerMgr.Status.State = phase
	l.updated = true
	l.requeue = true
}

// StatusUpdate sets the layer manager's status.
func (l *KraanManager) StatusUpdate(phase, reason, message string) {
	l.setStatus(phase, reason, message)
}

// IsHold returns hold status.
func (l *KraanManager) IsHold() bool {
	return l.LayerMgr.Spec.Hold
}

// SetPrune sets the phase to prune.
func (l *KraanManager) SetPrune() {
	l.setStatus(kraanv1alpha1.PrunePhaseCondition,
		kraanv1alpha1.LayerMgrPruneReason, kraanv1alpha1.LayerMgrPruneMsg)
}

// SetDeploy sets the phase to deploy.
func (l *KraanManager) SetDeploy() {
	l.setStatus(kraanv1alpha1.ApplyPhaseCondition,
		kraanv1alpha1.LayerMgrDeployReason, kraanv1alpha1.LayerMgrDeployMsg)
}

// SetDone sets the phase to doney.
func (l *KraanManager) SetDone() {
	l.setStatus(kraanv1alpha1.DonePhaseCondition,
		kraanv1alpha1.LayerMgrDoneReason, kraanv1alpha1.LayerMgrDoneMsg)
}

// IsUpdated returns true if an update to the LayerMgr data has occurred.
func (l *KraanManager) IsUpdated() bool {
	return l.updated
}

// IsDelayed returns true if the requeue should be delayed.
func (l *KraanManager) IsDelayed() bool {
	return l.delayed
}

// NeedsRequeue returns true if the LayerMgr needed to be reprocessed.
func (l *KraanManager) NeedsRequeue() bool {
	return l.requeue
}

// GetDelay returns the delay period.
func (l *KraanManager) GetDelay() time.Duration {
	return l.delay
}

// GetStatus returns the status.
func (l *KraanManager) GetStatus() string {
	return l.LayerMgr.Status.State
}

// SetHold sets the hold status.
func (l *KraanManager) SetHold() {
	if l.IsHold() && l.GetStatus() != kraanv1alpha1.HoldCondition {
		l.StatusUpdate(kraanv1alpha1.HoldCondition,
			kraanv1alpha1.LayerMgrHoldReason, kraanv1alpha1.LayerMgrHoldMsg)
		l.updated = true
	}
}

// GetContext gets the context.
func (l *KraanManager) GetContext() context.Context {
	return l.ctx
}

// GetLogger gets the manager logger.
func (l *KraanManager) GetLogger() logr.Logger {
	return l.log
}

// GetName gets the manager name.
func (l *KraanManager) GetName() string {
	return l.LayerMgr.ObjectMeta.Name
}

// GetLayerMgrs returns a map containing the current state of all other LayerMgrs.
func (l *KraanManager) GetLayerMgrs() (map[string]*kraanv1alpha1.LayerMgr, error) {
	list := &kraanv1alpha1.LayerMgrList{}
	opt := &client.ListOptions{}
	if err := l.client.List(l.GetContext(), list, opt); err != nil {
		return nil, err
	}
	LayerMgrs := map[string]*kraanv1alpha1.LayerMgr{}
	for _, item := range list.Items {
		LayerMgrs[item.ObjectMeta.Name] = &item // nolint:scopelint,exportloopref // should be ok
	}
	return LayerMgrs, nil
}
