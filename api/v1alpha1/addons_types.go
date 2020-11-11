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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MaxConditions is the maximum number of condtions to retain.
var MaxConditions = 10

// PreReqs defines the prerequisites for an operation to be executed.
type PreReqs struct {
	// The minimum version of K8s to be deployed
	// +optional
	K8sVersion string `json:"k8sVersion"`

	// The names of other addons the addons depend on
	// +optional
	DependsOn []string `json:"dependsOn,omitempty"`

	// Add more prerequisites in the future.
}

// SourceSpec defines a source location using the source types supported by the GitOps Toolkit source controller.
type SourceSpec struct {
	// The name of the gitrepositories.source.fluxcd.io custom resource to use
	// +required
	Name string `json:"name"`

	// The namespace of the gitrepositories.source.fluxcd.io custom resource to use
	// +optional
	NameSpace string `json:"namespace"`

	// Path to the directory in the git repository to use, defaults to repository base directory.
	// The Kraan controller will process the yaml files in that directory.
	// +kubebuilder:validation:Pattern="^\\./"
	// +required
	Path string `json:"path"`
}

// AddonsLayerSpec defines the desired state of AddonsLayer.
type AddonsLayerSpec struct {
	// The source to obtain the addons definitions from
	// +required
	Source SourceSpec `json:"source"`

	// The prerequisites information, if not present not prerequisites
	// +optional
	PreReqs PreReqs `json:"prereqs,omitempty"`

	// This flag tells the controller to hold off deployment of these addons,
	// +optional
	Hold bool `json:"hold,omitempty"`

	// The interval at which to check for changes.
	// Defaults to controller's default
	// +optional
	Interval metav1.Duration `json:"interval"`

	// Timeout for operations.
	// Defaults to 'Interval' duration.
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// Version is the version of the addon layer
	// +required
	Version string `json:"version"`
}

// Condition contains condition information for an AddonLayer.
/*
Copyright 2020 The Flux CD contributors.

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
type Condition struct {
	// Type of the condition, currently ('Ready').
	// +required
	Type string `json:"type"`

	// Status of the condition, one of ('True', 'False', 'Unknown').
	// +required
	Status corev1.ConditionStatus `json:"status"`

	// Version, the version the status relates to.
	// +required
	Version string `json:"version,omitempty"`

	// LastTransitionTime is the timestamp corresponding to the last status
	// change of this condition.
	// +required
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`

	// Reason is a brief machine readable explanation for the condition's last
	// transition.
	// +required
	Reason string `json:"reason,omitempty"`

	// Message is a human readable description of the details of the last
	// transition, complementing reason.
	// +optional
	Message string `json:"message,omitempty"`
}

const (
	// K8sVersionCondition represents the fact that the addons layer is waiting for the required k8s Version.
	K8sVersionCondition string = "K8sVersion"

	// PruningCondition represents the fact that the addons are being pruned.
	PruningCondition string = "Pruning"

	// PendingCondition represents the fact that the addons are pending being processed.
	// Used when source is not ready.
	PendingCondition string = "Pending"

	// ApplyPendingCondition represents the fact that the addons are pending being applied.
	// Used when applying is pending a prereq being met.
	ApplyPendingCondition string = "ApplyPending"

	// ApplyingCondition represents the fact that the addons are being deployed.
	ApplyingCondition string = "Applying"

	// DeployedCondition represents the fact that the addons are deployed.
	DeployedCondition string = "Deployed"

	// FailedCondition represents the fact that the procesing of the addons failed.
	FailedCondition string = "Failed"

	// HoldCondition represents the fact that addons are on hold.
	HoldCondition string = "Hold"
)

// AddonsLayerStatus defines the observed status.
type AddonsLayerStatus struct {
	// Conditions history.
	// +optional
	Conditions []Condition `json:"conditions,omitempty"`

	// State is the current state of the layer.
	// +required
	State string `json:"state,omitempty"`

	// Version, the version the state relates to.
	// +required
	Version string `json:"version,omitempty"`

	// ObservedGeneration is the last reconciled generation.
	// +required
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// DeployedRevision is the source revsion that has been deployed.
	// +required
	DeployedRevision string `json:"revision"`
}

const (
	// AddonsLayerK8sVersionReason represents the addons layer is wating for the required K8s Version.
	AddonsLayerK8sVersionReason string = "AddonsLayer is waiting for the required K8sVersion"

	// AddonsLayerK8sVersionMsg explains that the addons k8s Version state.
	AddonsLayerK8sVersionMsg string = ("The k8sVersion status means the manager has detected" +
		" that the AddonsLayer needs a higher version of the Kubernetes API than the current" +
		" version running on the cluster.")

	// AddonsLayerPruningReason represents the fact that the addons are being pruned.
	AddonsLayerPruningReason string = "AddonsLayer is being pruned"

	// AddonsLayerPruningMsg explains that pruning status.
	AddonsLayerPruningMsg string = "The pruning status means the manager is pruning objects removed from this layer"

	// AddonsLayerApplyPendingReason represents the fact that the applying of addons is pending.
	AddonsLayerApplyPendingReason string = "Deployment of the AddonsLayer is pending because layers it is depends on are not deployed"

	// AddonsLayerApplyPendingMsg explains that the addons ApplyPending state.
	AddonsLayerApplyPendingMsg string = ("The ApplyPending status means the manager has detected that the AddonsLayer" +
		" needs to be applied a layer that ome or more layers depends on has not been applied yet")

	// AddonsLayerApplyingReason represents the fact that the addons are being deployed.
	AddonsLayerApplyingReason string = "AddonsLayer is being applied"

	// AddonsLayerApplyingMsg explains that the addons are being deployed.
	AddonsLayerApplyingMsg string = ("The applying status means the manager is either applying the yaml files" +
		" or waiting for the HelmReleases to successfully deploy.")

	// AddonsLayerFailedReason represents the fact that the deployment of the addons failed.
	AddonsLayerFailedReason string = "AddonsLayer processsing has failed"

	// AddonsLayerHoldReason represents the fact that addons are on hold.
	AddonsLayerHoldReason string = "AddonsLayer is on hold, preventing execution"

	// AddonsLayerHoldMsg explains the hold status.
	AddonsLayerHoldMsg string = ("AddonsLayer custom resource has the 'Hold' element set to ture" +
		" preventing it from being processed. To clear this state update the custom resource object" +
		" setting Hold to false")

	// AddonsLayerDeployedReason represents the fact that the addons has been successfully deployed.
	AddonsLayerDeployedReason string = "AddonsLayer is Deployed"
)

// AddonsLayer is the Schema for the addons API.
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=al;layer;addonlayer
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.spec.version`
// +kubebuilder:printcolumn:name="Source",type=string,JSONPath=`.spec.source.name`
// +kubebuilder:printcolumn:name="Path",type=string,JSONPath=`.spec.source.path`
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.state",description=""
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.status==\"True\")].reason",description=""
type AddonsLayer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AddonsLayerSpec   `json:"spec,omitempty"`
	Status AddonsLayerStatus `json:"status,omitempty"`
}

// AddonsLayerList contains a list of AddonsLayer.
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
type AddonsLayerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AddonsLayer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AddonsLayer{}, &AddonsLayerList{})
}
