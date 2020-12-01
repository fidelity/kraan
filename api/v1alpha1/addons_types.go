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
	// The kind of the resource to use, currently only supports gitrepositories.source.toolkit.fluxcd.io
	// +optional
	Kind string `json:"kind"`
	// The name of the resource to use
	// +required
	Name string `json:"name"`

	// The namespace of the resource to use
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
	Interval *metav1.Duration `json:"interval"`

	// Timeout for operations.
	// Defaults to 'Interval' duration.
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// Version is the version of the addon layer
	// +required
	Version string `json:"version"`
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

	// DeletedCondition represents the fact that the addons layer has been deleted.
	DeletedCondition string = "Deleted"

	// NotDeployed represents resource status of present in layer source but not deployed on the cluster
	NotDeployed string = "NotDeployed"

	// Deployed represents resource status of deployed on the cluster
	Deployed string = "Deployed"

	// Name of finalizer
	AddonsFinalizer = "finalizers.kraan.io"

	// AddonsLayerKind is the string representation of a AddonsLayer.
	AddonsLayerKind = "AddonsLayer"
)

type Resource struct {
	// Namespace of resource.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Name of resource.
	// +required
	Name string `json:"name"`

	// Kind of the resource.
	// +required
	Kind string `json:"kind"`

	// LastTransitionTime is the timestamp corresponding to the last status
	// change of this resource.
	// +required
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`

	// Status of the resource.
	// +required
	Status string `json:"status"`
}

type Resources []Resource

func (r Resources) Len() int      { return len(r) }
func (r Resources) Swap(i, j int) { r[i], r[j] = r[j], r[i] }
func (r Resources) Less(i, j int) bool {
	if r[i].Namespace == r[j].Namespace {
		return r[i].Name < r[j].Name
	}
	return r[i].Namespace < r[j].Namespace
}

// AddonsLayerStatus defines the observed status.
type AddonsLayerStatus struct {
	// Conditions history.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

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

	// Resources is a list of resources managed by this layer.
	// +optional
	Resources []Resource `json:"resources"`
}

const (
	// AddonsLayerK8sVersionMsg represents the addons layer is wating for the required K8s Version.
	AddonsLayerK8sVersionMsg string = "AddonsLayer is waiting for the required K8sVersion"

	// AddonsLayerPruningMsg represents the fact that the addons are being pruned.
	AddonsLayerPruningMsg string = "AddonsLayer is being pruned"

	// AddonsLayerApplyPendingMsg represents the fact that the applying of addons is pending.
	AddonsLayerApplyPendingMsg string = "Deployment of the AddonsLayer is pending because layers it is depends on are not deployed"

	// AddonsLayerApplyingMsg represents the fact that the addons are being deployed.
	AddonsLayerApplyingMsg string = "AddonsLayer is being applied"

	// AddonsLayerFailedMsg represents the fact that the deployment of the addons failed.
	AddonsLayerFailedMsg string = "AddonsLayer failed"

	// AddonsLayerHoldMsg represents the fact that addons are on hold.
	AddonsLayerHoldMsg string = "AddonsLayer is on hold, preventing execution"

	// AddonsLayerDeployedMsg represents the fact that the addons has been successfully deployed.
	AddonsLayerDeployedMsg string = "HelmReleases in AddonsLayer are Deployed"
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
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[?(@.status==\"True\")].message",description=""
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
