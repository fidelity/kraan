//Package layers provides an interface for processing AddonsLayers.
//go:generate mockgen -destination=mockLayers.go -package=layers -source=layers.go . Layer
package layers

import (
	"context"
	"fmt"
	"strings"
	"time"

	kraanv1alpha1 "github.com/fidelity/kraan/pkg/api/v1alpha1"
	"github.com/fidelity/kraan/pkg/internal/utils"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MaxConditions is the maximum number of condtions to retain.
var MaxConditions = 10
var rootPath = "/repos"

// Layer defines the interface for managing the layer.
type Layer interface {
	SetStatusK8sVersion()
	SetStatusApplying()
	SetStatusPruning()
	SetStatusDeployed()
	StatusUpdate(status, reason, message string)

	IsHold() bool
	SetHold()
	IsPruningRequired() bool
	Prune() error
	DependenciesDeployed() bool
	IsApplyRequired() bool
	SuccessfullyApplied() bool
	Apply() error

	GetStatus() string
	GetName() string
	GetLogger() logr.Logger
	GetContext() context.Context
	GetSourcePath() string
	GetTimeout() time.Duration
	IsUpdated() bool
	NeedsRequeue() bool
	IsDelayed() bool
	GetDelay() time.Duration
	SetRequeue()
	SetDelayedRequeue()
	SetUpdated()
	GetRequiredK8sVersion() string
	CheckK8sVersion() bool
	GetFullStatus() *kraanv1alpha1.AddonsLayerStatus
	GetSpec() *kraanv1alpha1.AddonsLayerSpec
	GetAddonsLayer() *kraanv1alpha1.AddonsLayer

	getOtherAddonsLayer(name string) (*kraanv1alpha1.AddonsLayer, error)
	getK8sClient() kubernetes.Interface
	setStatus(status, reason, message string)
	isOtherDeployed(otherVersion string, otherLayer *kraanv1alpha1.AddonsLayer) bool
}

// KraanLayer is the Schema for the addons API.
type KraanLayer struct {
	updated     bool
	requeue     bool
	delayed     bool
	delay       time.Duration
	ctx         context.Context
	client      client.Client
	k8client    kubernetes.Interface
	log         logr.Logger
	Layer       `json:"-"`
	addonsLayer *kraanv1alpha1.AddonsLayer
}

// CreateLayer creates a layer object.
func CreateLayer(ctx context.Context, client client.Client, k8client kubernetes.Interface,
	log logr.Logger, addonsLayer *kraanv1alpha1.AddonsLayer) Layer {
	l := &KraanLayer{
		requeue:     false,
		delayed:     false,
		updated:     false,
		ctx:         ctx,
		client:      client,
		k8client:    k8client,
		log:         log,
		addonsLayer: addonsLayer,
	}
	l.delay = l.addonsLayer.Spec.Interval.Duration
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

// SetDelayedRequeue sets the delayed flag to cause the AddonsLayer to delay the requeue.
func (l *KraanLayer) SetDelayedRequeue() {
	l.delayed = true
	l.requeue = true
}

// GetFullStatus returns the AddonsLayers Status sub resource.
func (l *KraanLayer) GetFullStatus() *kraanv1alpha1.AddonsLayerStatus {
	return &l.addonsLayer.Status
}

// GetSpec returns the AddonsLayers Spec.
func (l *KraanLayer) GetSpec() *kraanv1alpha1.AddonsLayerSpec {
	return &l.addonsLayer.Spec
}

// GetAddonsLayer returns the AddonsLayers Spec.
func (l *KraanLayer) GetAddonsLayer() *kraanv1alpha1.AddonsLayer {
	return l.addonsLayer
}

// GetRequiredK8sVersion returns the K8s Version required.
func (l *KraanLayer) GetRequiredK8sVersion() string {
	return l.addonsLayer.Spec.PreReqs.K8sVersion
}

func (l *KraanLayer) getK8sClient() kubernetes.Interface {
	return l.k8client
}

// CheckK8sVersion checks if the cluster api server version is equal to or above the required version.
func (l *KraanLayer) CheckK8sVersion() bool {
	// TODO - research how to obtain this information using the generic REST client and remove the
	//        kubernetes.Clientset management functions from the main and addons_controller go files.
	versionInfo, err := l.getK8sClient().Discovery().ServerVersion()
	if err != nil {
		utils.LogError(l.GetLogger(), 2, err, "failed get server version")
		l.StatusUpdate(l.GetStatus(), "failed to obtain cluster api server version",
			err.Error())
		l.SetDelayedRequeue()
		return false
	}
	return versionInfo.String() > l.GetRequiredK8sVersion()
}

func (l *KraanLayer) trimConditions() {
	length := len(l.addonsLayer.Status.Conditions)
	if length < MaxConditions {
		return
	}
	trimedCond := l.addonsLayer.Status.Conditions[length-MaxConditions:]
	l.addonsLayer.Status.Conditions = trimedCond
}

func (l *KraanLayer) setStatus(status, reason, message string) {
	length := len(l.addonsLayer.Status.Conditions)
	if length > 0 {
		last := l.addonsLayer.Status.Conditions[length-1]
		if last.Reason == reason && last.Message == message && last.Type == status {
			return
		}
	}

	l.addonsLayer.Status.Conditions = append(l.addonsLayer.Status.Conditions, kraanv1alpha1.Condition{
		Type:               status,
		Version:            l.addonsLayer.Spec.Version,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	})
	l.trimConditions()
	l.addonsLayer.Status.State = status
	l.addonsLayer.Status.Version = l.addonsLayer.Spec.Version
	l.updated = true
	l.requeue = true
}

// SetStatusK8sVersion sets the addon layer's status to waiting for required K8s Version.
func (l *KraanLayer) SetStatusK8sVersion() {
	l.setStatus(kraanv1alpha1.K8sVersionCondition,
		kraanv1alpha1.AddonsLayerK8sVersionReason, kraanv1alpha1.AddonsLayerK8sVersionMsg)
}

// SetStatusDeployed sets the addon layer's status to deployed.
func (l *KraanLayer) SetStatusDeployed() {
	l.setStatus(kraanv1alpha1.DeployedCondition,
		kraanv1alpha1.AddonsLayerDeployedReason, "")
}

// SetStatusApplying sets the addon layer's status to apply in progress.
func (l *KraanLayer) SetStatusApplying() {
	l.setStatus(kraanv1alpha1.ApplyingCondition,
		kraanv1alpha1.AddonsLayerApplyingReason, kraanv1alpha1.AddonsLayerApplyingMsg)
}

// SetStatusPruning sets the addon layer's status to pruning.
func (l *KraanLayer) SetStatusPruning() {
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

// IsPruningRequired checks if there are any objects owned by the addons layer on the cluster that need to be pruned.
func (l *KraanLayer) IsPruningRequired() bool {
	return false
}

func getNameVersion(nameVersion string) (string, string) {
	parts := strings.Split(nameVersion, "@")
	if len(parts) < 2 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

func (l *KraanLayer) isOtherDeployed(otherVersion string, otherLayer *kraanv1alpha1.AddonsLayer) bool {
	if otherLayer.Spec.Version != otherVersion {
		reason := fmt.Sprintf("waiting for layer: %s, version: %s to be applied.", otherLayer.ObjectMeta.Name, otherVersion)
		message := fmt.Sprintf("Layer: %s, current version is: %s, require version: %s.",
			otherLayer.ObjectMeta.Name, otherLayer.Spec.Version, otherVersion)
		l.setStatus(kraanv1alpha1.ApplyPendingCondition, reason, message)
		return false
	}
	if otherLayer.Status.State != kraanv1alpha1.DeployedCondition {
		reason := fmt.Sprintf("waiting for layer: %s, version: %s to be applied.", otherLayer.ObjectMeta.Name, otherVersion)
		message := fmt.Sprintf("Layer: %s, current state: %s.", otherLayer.ObjectMeta.Name, otherLayer.Status.State)
		l.setStatus(kraanv1alpha1.ApplyPendingCondition, reason, message)
		return false
	}
	return true
}

// DependenciesDeployed checks that all the layers this layer is dependent on are deployed.
func (l *KraanLayer) DependenciesDeployed() bool {
	for _, otherNameVersion := range l.GetSpec().PreReqs.DependsOn {
		otherName, otherVersion := getNameVersion(otherNameVersion)
		otherLayer, err := l.getOtherAddonsLayer(otherName)
		if err != nil {
			l.StatusUpdate(kraanv1alpha1.FailedCondition, kraanv1alpha1.AddonsLayerFailedReason, err.Error())
			return false
		}
		if !l.isOtherDeployed(otherVersion, otherLayer) {
			return false
		}
	}
	return true
}

// IsApplyRequired checks if an apply is required.
func (l *KraanLayer) IsApplyRequired() bool {
	// Not yet implemented, for now return true if it is not in applying status otherrwise true
	// This forces the processing to set applying status wait for dependencies and the apply
	// at which point the applying condition is set so we can progress
	return l.GetStatus() != kraanv1alpha1.ApplyingCondition && l.GetStatus() != kraanv1alpha1.DeployedCondition
}

// SuccessfullyApplied checks if all Helm Releases in a layer have beem successfully applied.
func (l *KraanLayer) SuccessfullyApplied() bool {
	return true
}

// Apply addons layer objects to cluster.
func (l *KraanLayer) Apply() error {
	return nil
}

// Prune deletes any objects owned by the addon layer on the cluster that are not defined in the addon layer anymore.
func (l *KraanLayer) Prune() error {
	return nil
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

// GetDelay returns the delay period.
func (l *KraanLayer) GetDelay() time.Duration {
	return l.delay
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
	return fmt.Sprintf("%s/%s/%s",
		rootPath,
		l.addonsLayer.Spec.Source.Name,
		l.addonsLayer.Spec.Source.Path)
}

// GetContext gets the context.
func (l *KraanLayer) GetContext() context.Context {
	return l.ctx
}

// GetLogger gets the layer logger.
func (l *KraanLayer) GetLogger() logr.Logger {
	return l.log
}

// GetName gets the layer name.
func (l *KraanLayer) GetName() string {
	return l.addonsLayer.ObjectMeta.Name
}

// GetAddonsLayers returns a map containing the current state of all AddonsLayers in this group.
func (l *KraanLayer) getOtherAddonsLayer(name string) (*kraanv1alpha1.AddonsLayer, error) {
	obj := &kraanv1alpha1.AddonsLayer{}
	if err := l.client.Get(l.GetContext(), types.NamespacedName{Name: name}, obj); err != nil {
		return nil, err
	}
	return obj, nil
}
