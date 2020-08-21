//Package layers xxx
package layers

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kraanv1alpha1 "github.com/fidelity/kraan/pkg/api/v1alpha1"
	"github.com/fidelity/kraan/pkg/internal/utils"
)

// MaxConditions is the maximum number of condtions to retain.
var MaxConditions = 10
var rootPath = "/repos"

// KraanLayer is the structure that hold details of the AddonsLayer.
type KraanLayer struct {
	updated     bool
	requeue     bool
	delayed     bool
	delay       time.Duration
	ctx         context.Context
	client      client.Client
	log         logr.Logger
	Layer       `json:"-"`
	addonsLayer *kraanv1alpha1.AddonsLayer
}

// Layer defines the interface for managing the layer.
type Layer interface {
	SetStatusApplying()
	SetStatusApply()
	SetStatusApplyPending()
	SetStatusPrunePending()
	SetStatusPruned()
	SetStatusPruning()
	SetStatusDeployed()
	StatusUpdate(status, reason, message string)
	setStatus(status, reason, message string)

	IsHold() bool
	SetHold()
	IsPruningRequired() bool
	Prune() error
	SetAllPrunePending() error
	SetStatusPruningToPruned()
	AllPruned() bool
	SetAllPrunedToApplyPending() error
	DependenciesDeployed() bool
	IsApplyRequired() bool
	Apply() error

	GetStatus() string
	GetName() string
	GetLogger() logr.Logger
	GetK8sClient() client.Client
	GetContext() context.Context
	GetSourcePath() string
	GetInterval() time.Duration
	GetTimeout() time.Duration
	IsUpdated() bool
	NeedsRequeue() bool
	IsVersionCurrent() bool
	IsDelayed() bool
	GetDelay() time.Duration
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
func CreateLayer(ctx context.Context, client client.Client,
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

// GetAddonsLayer returns the AddonsLayers Spec.
func (l *KraanLayer) GetAddonsLayer() *kraanv1alpha1.AddonsLayer {
	return l.addonsLayer
}

// GetRequiredK8sVersion returns the K8s Version required.
func (l *KraanLayer) GetRequiredK8sVersion() string {
	return l.addonsLayer.Spec.PreReqs.K8sVersion
}

// CheckK8sVersion checks if the cluster api server version is equal to or above the required version.
func (l *KraanLayer) CheckK8sVersion() bool {
	versionInfo, err := getK8sClient().Discovery().ServerVersion()
	if err != nil {
		utils.LogError(l.GetLogger(), 2, err, "failed get server version")
		l.StatusUpdate(l.GetStatus(), "failed to obtain cluster api server version",
			err.Error())
		l.SetDelayed()
		return false
	}
	return versionInfo.String() > l.GetRequiredK8sVersion()
}

// GetK8sClient gets the Kubernetes client.
func getK8sClient() *kubernetes.Clientset {
	kubeConfig := os.Getenv("KUBECONFIG")
	if len(kubeConfig) > 0 {
		// use the current context in kubeconfig
		config, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
		if err != nil {
			panic(err.Error())
		}

		// create the clientset
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			panic(err.Error())
		}

		return clientset
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

	return clientset
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
	l.requeue = true
}

// SetStatusDeployed sets the addon layer's status to deployed.
func (l *KraanLayer) SetStatusDeployed() {
	if l.GetStatus() != kraanv1alpha1.DeployedCondition {
		l.setStatus(kraanv1alpha1.DeployedCondition,
			kraanv1alpha1.AddonsLayerDeployedReason, "")
	}
}

// SetStatusApplyPending sets the addon layer's status to apply pending.
func (l *KraanLayer) SetStatusApplyPending() {
	l.setStatus(kraanv1alpha1.ApplyPendingCondition,
		kraanv1alpha1.AddonsLayerApplyPendingReason, kraanv1alpha1.AddonsLayerApplyPendingMsg)
}

// SetStatusApplying sets the addon layer's status to apply in progress.
func (l *KraanLayer) SetStatusApplying() {
	l.setStatus(kraanv1alpha1.ApplyingCondition,
		kraanv1alpha1.AddonsLayerApplyingReason, kraanv1alpha1.AddonsLayerApplyingMsg)
}

// SetStatusPrunePending sets the addon layer's status to prune pending.
func (l *KraanLayer) SetStatusPrunePending() {
	l.setStatus(kraanv1alpha1.PrunePendingCondition,
		kraanv1alpha1.AddonsLayerPrunePendingReason, kraanv1alpha1.AddonsLayerPrunePendingMsg)
}

// SetStatusPruned sets the addon layer's status to prune.
func (l *KraanLayer) SetStatusPruned() {
	l.setStatus(kraanv1alpha1.PrunedCondition,
		kraanv1alpha1.AddonsLayerPrunedReason, kraanv1alpha1.AddonsLayerPrunedMsg)
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

// IsVersionCurrent returns true if the spec version matches the status version.
func (l *KraanLayer) IsVersionCurrent() bool {
	return l.addonsLayer.Spec.Version == l.addonsLayer.Status.Version
}

// IsPruningRequired checks if there are any objects owned by the addons layer on the cluster that need to be pruned.
func (l *KraanLayer) IsPruningRequired() bool {
	return false
}

// SetAllPrunePending sets all addons layer custom  resources to prune pending status.
func (l *KraanLayer) SetAllPrunePending() error {
	return nil
}

// SetStatusPruningToPruned sets the status to pruned if it is currently pruning.
func (l *KraanLayer) SetStatusPruningToPruned() {
	if l.GetStatus() == kraanv1alpha1.PruningCondition {
		l.SetStatusPruned()
	}
}

// AllPruned checks if all AddonsLayer custom resources are in the pruned status.
func (l *KraanLayer) AllPruned() bool {
	return false
}

// DependenciesDeployed checks that all the layers this layer is dependent on are deployed.
func (l *KraanLayer) DependenciesDeployed() bool {
	return false
}

// IsApplyRequired checks if an apply is required.
func (l *KraanLayer) IsApplyRequired() bool {
	return false
}

// Apply addons layer objects to cluster.
func (l *KraanLayer) Apply() error {
	return nil
}

// SetAllPrunedToApplyPending sets all AddonsLayer custom resources  in the pruned status to apply pending status.
func (l *KraanLayer) SetAllPrunedToApplyPending() error {
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

// GetInterval returns the interval.
func (l *KraanLayer) GetInterval() time.Duration {
	return l.addonsLayer.Spec.Interval.Duration
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
func (l *KraanLayer) GetK8sClient() client.Client {
	return l.client
}

// GetLogger gets the layer logger.
func (l *KraanLayer) GetLogger() logr.Logger {
	return l.log
}

// GetName gets the layer name.
func (l *KraanLayer) GetName() string {
	return l.addonsLayer.ObjectMeta.Name
}
