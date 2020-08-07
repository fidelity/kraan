package layers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	kraanv1alpha1 "github.com/fidelity/kraan/pkg/api/v1alpha1"
	"github.com/fidelity/kraan/pkg/internal/utils"
)

// MaxConditions is the maximum number of condtions to retain.
var MaxConditions = 10
var rootPath = "/repos"

// Layer is the Schema for the addons API.
type Layer struct {
	updated     bool
	requeue     bool
	delayed     bool
	delay       time.Duration
	ctx         context.Context
	client      *kubernetes.Clientset
	log         logr.Logger
	LayerI      `json:"-"`
	addonsLayer *kraanv1alpha1.AddonsLayer
}

// LayerI defines the interface for managing the layer.
type LayerI interface {
	StatusReady()
	StatusApplying()
	StatusApply()
	StatusApplyPending()
	StatusPrunePending()
	StatusPrune()
	StatusPruning()
	StatusDeployed()
	IsHold() bool
	SetHold()
	GetStatus() string
	GetName() string
	GetNamespace() string
	GetLogger() logr.Logger
	GetK8sClient() *kubernetes.Clientset
	GetContext() context.Context
	GetSourcePath() string
	IsReadyToProcess() bool
	GetInterval() time.Duration
	GetTimeout() time.Duration
	IsUpdated() bool
	NeedsRequeue() bool
	IsVersionCurrent() bool
	IsDelayed() bool
	GetDelay() time.Duration
	CheckPreReqs()
	SetRequeue()
	SetDelayed()
	SetUpdated()
	GetRequiredK8sVersion() string
	CheckK8sVersion() bool
	GetFullStatus() *kraanv1alpha1.AddonsLayerStatus
	GetSpec() *kraanv1alpha1.AddonsLayerSpec
	GetAddonsLayer() *kraanv1alpha1.AddonsLayer
}

// CreateLayer creates a layer object.
func CreateLayer(ctx context.Context, client *kubernetes.Clientset,
	log logr.Logger, addonsLayer *kraanv1alpha1.AddonsLayer) *Layer {
	l := &Layer{requeue: false, delayed: false, updated: false, ctx: ctx, client: client, log: log,
		addonsLayer: addonsLayer}
	l.delay = l.GetInterval()
	return l
}

// SetRequeue sets the requeue flag to cause the AddonsLayer to be requeued.
func (l *Layer) SetRequeue() {
	l.requeue = true
}

// SetUpdated sets the updated flag to cause the AddonsLayer to update the custom resource.
func (l *Layer) SetUpdated() {
	l.updated = true
}

// SetDelayed sets the delayed flag to cause the AddonsLayer to delay the requeue.
func (l *Layer) SetDelayed() {
	l.delayed = true
}

// GetFullStatus returns the AddonsLayers Status sub resource.
func (l *Layer) GetFullStatus() *kraanv1alpha1.AddonsLayerStatus {
	return &l.addonsLayer.Status
}

// GetSpec returns the AddonsLayers Spec.
func (l *Layer) GetSpec() *kraanv1alpha1.AddonsLayerSpec {
	return &l.addonsLayer.Spec
}

// GetSpec returns the AddonsLayers Spec.
func (l *Layer) GetAddonsLayer() *kraanv1alpha1.AddonsLayer {
	return l.addonsLayer
}

// IsReadyToProcess returns a boolean if an AddonsLayer is ready to be processed.
func (l *Layer) IsReadyToProcess() bool {
	if l.IsHold() {
		l.SetHold()
		return false
	}

	if l.GetStatus() == kraanv1alpha1.DeployedCondition {
		// It is in ready status so return true
		return true
	}

	if l.GetStatus() == kraanv1alpha1.PrunePendingCondition {
		// waiting, recheck prereqs
		l.CheckPreReqs()
		return false
	}
	return false
}

// GetRequiredK8sVersion returns the K8s Version required.
func (l *Layer) GetRequiredK8sVersion() string {
	return l.addonsLayer.Spec.PreReqs.K8sVersion
}

// CheckK8sVersion checks if the cluster api server version is equal to or above the required version.
func (l *Layer) CheckK8sVersion() bool {
	// if still not ready, requeue
	l.SetRequeue()
	versionInfo, err := l.GetK8sClient().Discovery().ServerVersion()
	if err != nil {
		utils.LogError(l.GetLogger(), 2, err, "failed get server version")
		l.StatusUpdate(l.GetStatus(), "failed to obtain cluster api server version",
			err.Error())
		l.SetDelayed()
		return false
	}
	return versionInfo.String() > l.GetRequiredK8sVersion()
}

/*
func (l *Layer) trimConditions() {
	for {
		if len(l.addonsLayer.Status.Conditions) > MaxConditions {
			t := l.addonsLayer.Status.Conditions[1:]
			l.addonsLayer.Status.Conditions = t // might losee order, will need a sort
		}
	}
}
*/

func (l *Layer) setStatus(status, reason, message string) {
	l.addonsLayer.Status.Conditions = append(l.addonsLayer.Status.Conditions, kraanv1alpha1.Condition{
		Type:               status,
		Version:            l.addonsLayer.Spec.Version,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	})
	//l.trimConditions()
	l.addonsLayer.Status.State = status
	l.addonsLayer.Status.Version = l.addonsLayer.Spec.Version
	l.updated = true
}

// StatusDeployed sets the addon layer's status to deployed.
func (l *Layer) StatusDeployed(reason, message string) {
	l.setStatus(kraanv1alpha1.DeployedCondition, reason, message)
}

// StatusApplyPending sets the addon layer's status to apply pending.
func (l *Layer) StatusApplyPending() {
	l.setStatus(kraanv1alpha1.ApplyPendingCondition,
		kraanv1alpha1.AddonsLayerApplyPendingReason, kraanv1alpha1.AddonsLayerApplyPendingMsg)
}

// StatusApply sets the addon layer's status to apply.
func (l *Layer) StatusApply() {
	l.setStatus(kraanv1alpha1.ApplyCondition,
		kraanv1alpha1.AddonsLayerApplyReason, kraanv1alpha1.AddonsLayerApplyMsg)
}

// StatusApplying sets the addon layer's status to apply in progress.
func (l *Layer) StatusApplying() {
	l.setStatus(kraanv1alpha1.ApplyingCondition,
		kraanv1alpha1.AddonsLayerApplyingReason, kraanv1alpha1.AddonsLayerApplyingMsg)
}

// StatusPrunePending sets the addon layer's status to prune pending.
func (l *Layer) StatusPrunePending() {
	l.setStatus(kraanv1alpha1.PrunePendingCondition,
		kraanv1alpha1.AddonsLayerPrunePendingReason, kraanv1alpha1.AddonsLayerPrunePendingMsg)
}

// StatusPrune sets the addon layer's status to prune.
func (l *Layer) StatusPrune() {
	l.setStatus(kraanv1alpha1.PruneCondition,
		kraanv1alpha1.AddonsLayerPruneReason, kraanv1alpha1.AddonsLayerPruneMsg)
}

// StatusPruning sets the addon layer's status to pruning.
func (l *Layer) StatusPruning() {
	l.setStatus(kraanv1alpha1.PruningCondition,
		kraanv1alpha1.AddonsLayerPruningReason, kraanv1alpha1.AddonsLayerPruningMsg)
}

// StatusUpdate sets the addon layer's status.
func (l *Layer) StatusUpdate(status, reason, message string) {
	l.setStatus(status, reason, message)
}

// IsHold returns hold status.
func (l *Layer) IsHold() bool {
	return l.addonsLayer.Spec.Hold
}

// IsVersionCurrent returns true if the spec version matches the status version.
func (l *Layer) IsVersionCurrent() bool {
	return l.addonsLayer.Spec.Version == l.addonsLayer.Status.Version
}

// IsUpdated returns true if an update to the AddonsLayer data has occurred.
func (l *Layer) IsUpdated() bool {
	return l.updated
}

// IsDelayed returns true if the requeue should be delayed.
func (l *Layer) IsDelayed() bool {
	return l.delayed
}

// NeedsRequeue returns true if the AddonsLayer needed to be reprocessed.
func (l *Layer) NeedsRequeue() bool {
	return l.requeue
}

// GetRequeueDelay returns the requeue delay.
func (l *Layer) GetRequeueDelay() time.Duration {
	return l.delay
}

// GetInterval returns the interval.
func (l *Layer) GetInterval() time.Duration {
	return l.addonsLayer.Spec.Interval.Duration
}

// GetStatus returns the status.
func (l *Layer) GetStatus() string {
	return l.addonsLayer.Status.State
}

// SetHold sets the hold status.
func (l *Layer) SetHold() {
	if l.IsHold() && l.GetStatus() != kraanv1alpha1.HoldCondition {
		l.StatusUpdate(kraanv1alpha1.HoldCondition,
			kraanv1alpha1.AddonsLayerHoldReason, kraanv1alpha1.AddonsLayerHoldMsg)
		l.updated = true
	}
}

// GetSourcePath gets the path to the addons layer's top directory in the local filesystem.
func (l *Layer) GetSourcePath() string {
	return fmt.Sprintf("%s/%s/%s/%s",
		rootPath,
		l.addonsLayer.Spec.Source.NameSpace,
		l.addonsLayer.Spec.Source.Name,
		l.addonsLayer.Spec.Source.Path)
}

// GetContext gets the context.
func (l *Layer) GetContext() context.Context {
	return l.ctx
}

// GetK8sClient gets the k8sClient.
func (l *Layer) GetK8sClient() *kubernetes.Clientset {
	return l.client
}

// GetLogger gets the layer logger.
func (l *Layer) GetLogger() logr.Logger {
	return l.log
}

// GetName gets the layer name.
func (l *Layer) GetName() string {
	return l.addonsLayer.ObjectMeta.Name
}

// GetNamespace gets the layer namespace.
func (l *Layer) GetNamespace() string {
	return l.addonsLayer.ObjectMeta.Namespace
}
