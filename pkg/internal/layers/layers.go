//Package layers xxx
//go:generate mockgen -destination=mockLayers.go -package=layers -source=layers.go . Layer
package layers

import (
	"context"
	"fmt"
	"time"

	helmrelease "github.com/fluxcd/helm-operator/pkg/client/clientset/versioned"
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

// Layer defines the interface for managing the layer.
type Layer interface {
	StatusReady()
	StatusApplying()
	StatusApply()
	StatusApplyPending()
	StatusPrunePending()
	StatusPrune()
	StatusPruning()
	StatusDeployed(reason, message string)
	IsHold() bool
	SetHold()
	GetStatus() string
	GetName() string
	GetNamespace() string
	GetLogger() logr.Logger
	GetK8sClient() *kubernetes.Clientset
	GetHelmReleaseClient() *helmrelease.Clientset
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

// KraanLayer is the Schema for the addons API.
type KraanLayer struct {
	updated     bool
	requeue     bool
	delayed     bool
	delay       time.Duration
	ctx         context.Context
	client      *kubernetes.Clientset
	hrclient    *helmrelease.Clientset
	log         logr.Logger
	Layer       `json:"-"`
	addonsLayer *kraanv1alpha1.AddonsLayer
}

// CreateLayer creates a layer object.
func CreateLayer(ctx context.Context, client *kubernetes.Clientset,
	log logr.Logger, addonsLayer *kraanv1alpha1.AddonsLayer) Layer {
	l := &KraanLayer{requeue: false, delayed: false, updated: false, ctx: ctx, client: client, log: log,
		addonsLayer: addonsLayer}
	l.delay = l.GetInterval()
	return l
}

// SetRequeue sets the requeue flag to cause the AddonsLayer to be requeued.
func (l *KraanLayer) SetRequeue() {
	l.requeue = true
}

// SetUpdated sets the updated flag to cause the AddonsLayer to update the custom resource.
func (l *KraanLayer) SetUpdated() {
	l.updated = true
}

// SetDelayed sets the delayed flag to cause the AddonsLayer to delay the requeue.
func (l *KraanLayer) SetDelayed() {
	l.delayed = true
}

// GetFullStatus returns the AddonsLayers Status sub resource.
func (l *KraanLayer) GetFullStatus() *kraanv1alpha1.AddonsLayerStatus {
	return &l.addonsLayer.Status
}

// GetSpec returns the AddonsLayers Spec.
func (l *KraanLayer) GetSpec() *kraanv1alpha1.AddonsLayerSpec {
	return &l.addonsLayer.Spec
}

// GetAddonsLayer returns the underlying AddonsLayer API type struct.
func (l *KraanLayer) GetAddonsLayer() *kraanv1alpha1.AddonsLayer {
	return l.addonsLayer
}

// IsReadyToProcess returns a boolean if an AddonsLayer is ready to be processed.
func (l *KraanLayer) IsReadyToProcess() bool {
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
func (l *KraanLayer) GetRequiredK8sVersion() string {
	return l.addonsLayer.Spec.PreReqs.K8sVersion
}

// CheckK8sVersion checks if the cluster api server version is equal to or above the required version.
func (l *KraanLayer) CheckK8sVersion() bool {
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
func (l *KraanLayer) trimConditions() {
	for {
		if len(l.addonsLayer.Status.Conditions) > MaxConditions {
			t := l.addonsLayer.Status.Conditions[1:]
			l.addonsLayer.Status.Conditions = t // might losee order, will need a sort
		}
	}
}
*/

func (l *KraanLayer) setStatus(status, reason, message string) {
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
func (l *KraanLayer) StatusDeployed(reason, message string) {
	l.setStatus(kraanv1alpha1.DeployedCondition, reason, message)
}

// StatusApplyPending sets the addon layer's status to apply pending.
func (l *KraanLayer) StatusApplyPending() {
	l.setStatus(kraanv1alpha1.ApplyPendingCondition,
		kraanv1alpha1.AddonsLayerApplyPendingReason, kraanv1alpha1.AddonsLayerApplyPendingMsg)
}

// StatusApply sets the addon layer's status to apply.
func (l *KraanLayer) StatusApply() {
	l.setStatus(kraanv1alpha1.ApplyCondition,
		kraanv1alpha1.AddonsLayerApplyReason, kraanv1alpha1.AddonsLayerApplyMsg)
}

// StatusApplying sets the addon layer's status to apply in progress.
func (l *KraanLayer) StatusApplying() {
	l.setStatus(kraanv1alpha1.ApplyingCondition,
		kraanv1alpha1.AddonsLayerApplyingReason, kraanv1alpha1.AddonsLayerApplyingMsg)
}

// StatusPrunePending sets the addon layer's status to prune pending.
func (l *KraanLayer) StatusPrunePending() {
	l.setStatus(kraanv1alpha1.PrunePendingCondition,
		kraanv1alpha1.AddonsLayerPrunePendingReason, kraanv1alpha1.AddonsLayerPrunePendingMsg)
}

// StatusPrune sets the addon layer's status to prune.
func (l *KraanLayer) StatusPrune() {
	l.setStatus(kraanv1alpha1.PruneCondition,
		kraanv1alpha1.AddonsLayerPruneReason, kraanv1alpha1.AddonsLayerPruneMsg)
}

// StatusPruning sets the addon layer's status to pruning.
func (l *KraanLayer) StatusPruning() {
	l.setStatus(kraanv1alpha1.PruningCondition,
		kraanv1alpha1.AddonsLayerPruningReason, kraanv1alpha1.AddonsLayerPruningMsg)
}

// StatusUpdate sets the addon layer's status.
func (l *KraanLayer) StatusUpdate(status, reason, message string) {
	l.setStatus(status, reason, message)
}

// IsHold returns hold status.
func (l *KraanLayer) IsHold() bool {
	return l.addonsLayer.Spec.Hold
}

// IsVersionCurrent returns true if the spec version matches the status version.
func (l *KraanLayer) IsVersionCurrent() bool {
	return l.addonsLayer.Spec.Version == l.addonsLayer.Status.Version
}

// IsUpdated returns true if an update to the AddonsLayer data has occurred.
func (l *KraanLayer) IsUpdated() bool {
	return l.updated
}

// IsDelayed returns true if the requeue should be delayed.
func (l *KraanLayer) IsDelayed() bool {
	return l.delayed
}

// NeedsRequeue returns true if the AddonsLayer needed to be reprocessed.
func (l *KraanLayer) NeedsRequeue() bool {
	return l.requeue
}

// GetRequeueDelay returns the requeue delay.
func (l *KraanLayer) GetRequeueDelay() time.Duration {
	return l.delay
}

// GetInterval returns the interval.
func (l *KraanLayer) GetInterval() time.Duration {
	return l.addonsLayer.Spec.Interval.Duration
}

// GetStatus returns the status.
func (l *KraanLayer) GetStatus() string {
	return l.addonsLayer.Status.State
}

// SetHold sets the hold status.
func (l *KraanLayer) SetHold() {
	if l.IsHold() && l.GetStatus() != kraanv1alpha1.HoldCondition {
		l.StatusUpdate(kraanv1alpha1.HoldCondition,
			kraanv1alpha1.AddonsLayerHoldReason, kraanv1alpha1.AddonsLayerHoldMsg)
		l.updated = true
	}
}

// GetSourcePath gets the path to the addons layer's top directory in the local filesystem.
func (l *KraanLayer) GetSourcePath() string {
	return fmt.Sprintf("%s/%s/%s/%s",
		rootPath,
		l.addonsLayer.Spec.Source.NameSpace,
		l.addonsLayer.Spec.Source.Name,
		l.addonsLayer.Spec.Source.Path)
}

// GetContext gets the context.
func (l *KraanLayer) GetContext() context.Context {
	return l.ctx
}

// GetK8sClient gets the k8sClient.
func (l *KraanLayer) GetK8sClient() *kubernetes.Clientset {
	return l.client
}

// GetHelmReleaeClient gets the HelmRelease API client.
func (l *KraanLayer) GetHelmReleaseClient() *helmrelease.Clientset {
	return l.hrclient
}

// GetLogger gets the layer logger.
func (l *KraanLayer) GetLogger() logr.Logger {
	return l.log
}

// GetName gets the layer name.
func (l *KraanLayer) GetName() string {
	return l.addonsLayer.ObjectMeta.Name
}

// GetNamespace gets the layer namespace.
func (l *KraanLayer) GetNamespace() string {
	return l.addonsLayer.ObjectMeta.Namespace
}
