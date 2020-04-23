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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EnvironmentSpec defines the desired state of an Environment.
type EnvironmentSpec struct {
	// Policy specifies what operator behavior is allowed.
	// Valid values are:
	// - "AllowAll" (default): allows create, update and delete of environment and clusters;
	// - "DenyDelete": forbids deletion of a cluster;
	// - "DenyUpdate": forbids update/delete of a cluster.
	// +optional
	Policy EnvironmentPolicy `json:"policy,omitempty"`

	// Defaults are the ClusterSpec's defaults.
	Defaults ClusterSpec `json:"defaults,omitempty"`

	// Clusters are the ClusterSpec's for each cluster instance.
	Clusters []ClusterSpec `json:"clusters,omitempty"`

	//TODO implement Test
}

// EnvironmentPolicy describes how the environment will be updated or deleted.
// Only one of the following policies may be specified.
// If none is specified, the default one AllowAll.
// +kubebuilder:validation:Enum=AllowAll;DenyDelete;DenyUpdate
type EnvironmentPolicy string

const (
	// AllowAll allows create, update and delete of cluster add-ons.
	AllowAll EnvironmentPolicy = "AllowAll"

	// DenyDelete forbids delete of cluster add-ons when ClusterAddon resource is deleted.
	DenyDelete EnvironmentPolicy = "DenyDelete"

	// DenyUpdate forbids update/delete of cluster add-ons when ClusterAddon or repo changes.
	DenyUpdate EnvironmentPolicy = "DenyUpdate"
)

type ClusterSpec struct {
	// Name is the cluster name.
	Name           string             `json:"name,omitempty"`
	Infrastructure InfrastructureSpec `json:"infrastructure,omitempty"`
	Addons         AddonSpec          `json:"addons,omitempty"`
	Test           TestSpec           `json:"test,omitempty"`
}

type InfrastructureSpec struct {
	// Source is the repository that contains Terraform infrastructure code.
	Source SourceSpec `json:"source,omitempty"`

	// Main is the path in the source tree to main.tf.
	Main string `json:"main,omitempty"`

	// Azure AD.
	AAD AADSpec `json:"aad,omitempty"`

	// AKS values.
	// +optional
	AKS AKSSpec `json:"aks,omitempty"`

	// X are extension values (when regular values don't fit the need)
	// +optional
	X map[string]string `json:"x,omitempty"`
	//TODO alternative is to use json.RawMessage see https://github.com/kubernetes-sigs/controller-tools/issues/294
}

// SourceSpec contains the data to fetch content from a remote source.
type SourceSpec struct {
	// Name is used to refer to this target when providing feedback to the user.
	//TODO not needed Name string `json:"name,omitempty"`

	// Type is the type of repository to use as a source.
	// Valid values are:
	// - "git" (default): GIT repository.
	// - "local": local filesystem.
	// +optional
	Type EnvironmentSourceType `json:"type,omitempty"`

	// URL is the URL of the repo.
	// When Token is specified the URL is expected to start with 'https://'.
	// +optional
	URL string `json:"url"`

	// Ref is the reference to the content to get.
	// For type=git it can be 'master', 'refs/heads/my-branch' etc, see 'git reference' doc.
	// +optional
	Ref string `json:"ref"`

	// Token is used to authenticate with the remote server.
	// For Type=git;
	// - Token or ~/.ssh key should be specified (azure devops requires the token to be prefixed with 'x:')
	// +optional
	Token string `json:"token,omitempty"`
}

// EnvironmentSourceType is the type of repository to use as a source.
// Valid values are:
// - SourceTypeGIT (default)
// - SourceTypeLocal
// +kubebuilder:validation:Enum=git;local
type EnvironmentSourceType string

const (
	// SourceTypeGIT specifies a source repository of type GIT.
	SourceTypeGIT EnvironmentSourceType = "git"
	// SourceTypeLocal specifies a source repository of type local filesystem.
	SourceTypeLocal EnvironmentSourceType = "local"
)

// Azure Active Directory.
type AADSpec struct {
	TenantID        string `json:"tenantID,omitempty"`
	ServerAppID     string `json:"serverAppID,omitempty"`
	ServerAppSecret string `json:"serverAppSecret,omitempty"`
	ClientAppID     string `json:"clientAppID,omitempty"`
}

type AKSSpec struct {
	/* Azure common values */

	Subscription  string `json:"subscription,omitempty"`
	ResourceGroup string `json:"resourceGroup,omitempty"`
	EnvDomain     string `json:"envDomain,omitempty"`
	VNetCIDR      string `json:"vnetCIDR,omitempty"`
	SubnetNewbits int    `json:"subnetNewbits,omitempty"`

	/* AKS Cluster values */

	// Cluster ordinal number.
	SubnetNum int `json:"subnetNewbits,omitempty"`
	// Kubernetes version.
	Version string `json:"version,omitempty"`
	// Cluster worker pools.
	// A 'default' pool is required.
	Pools map[string]AKSNodepoolSpec `json:"pools,omitempty"`
}

type AKSNodepoolSpec struct {
	// Number of VM's.
	Scale int `json:"subnetNewbits,omitempty"`
	// Type of VM.
	VMSize string `json:"subnetNewbits,omitempty"`
}

type AddonSpec struct {
	// Source is the repository that contains the k8s addons resources.
	Source SourceSpec `json:"source,omitempty"`

	// Jobs is an array of paths to job files in the source tree.
	Jobs []string `json:"jobs,omitempty"`

	// Values are key-value pairs that are passed as values to the job.
	// +optional
	Values map[string]string `json:"values,omitempty"`
}

type TestSpec struct {
	//TODO implement TestSpec
}

// EnvironmentStatus defines the observed state of an Environment.
type EnvironmentStatus struct {
	// Synced is a single word describing the Environment's fitness.
	// Check conditions for more details.
	//TODO consider renaming to summary
	// +optional
	Synced EnvironmentSyncedType `json:"synced,omitempty"`

	// Conditions are the latest available observations of the Environment's fitness.
	// +optional
	Conditions []EnvironmentCondition `json:"conditions,omitempty"`

	// Infra contains the deployment state of the infra and optional result values.
	// Note: cluster specific result values are in Clusters.
	// +optional
	Infra InfraStatus `json:"infra,omitempty"`

	// Clusters contains the deployment state of the clusters and optional result values.
	// +optional
	Clusters map[string]ClusterStatus `json:"clusters,omitempty"`

	// Clusters contains the deployment status of the clusters and optional result values.
	// +optional
	//TODO Tests map[string]TestStatus `json:"tests,omitempty"`
}

// EnvironmentSyncedType is the condition of the Environment in one word.
type EnvironmentSyncedType string

const (
	// SyncedUnknown means the Environment state is unknown.
	SyncedUnknown EnvironmentSyncedType = ""
	// EnvironmentSyncedRunning means changes are being made to get the Environment in the desired state.
	SyncedSyncing EnvironmentSyncedType = "Syncing"
	// SyncedReady means the Enviroment is in the desired state and there is nothing to do.
	SyncedReady EnvironmentSyncedType = "Ready"
	// SyncedError means an error occurred during syncing and human intervention is needed to recover.
	SyncedError EnvironmentSyncedType = "Error"
)

type InfraStatus struct {
	// PAdded is the number of infrastructure objects planned to being added on apply.
	// +optional
	PAdded int `json:"pAdded,omitempty"`
	// PChanged is the number of infrastructure objects planned to being changed on apply.
	// +optional
	PChanged int `json:"pChanged,omitempty"`
	// PAdded is the number of infrastructure objects planned to being deleted on apply.
	// +optional
	PDeleted int `json:"pDeleted,omitempty"`

	// Added is the number of infrastructure objects added on last apply.
	// +optional
	Added int `json:"added,omitempty"`
	// Changed is the number of infrastructure objects changed on last apply.
	// +optional
	Changed int `json:"changed,omitempty"`
	// Added is the number of infrastructure objects deleted on last apply.
	// +optional
	Deleted int `json:"deleted,omitempty"`

	// Hash is an unique value for the (terraform) source and parameters being deployed.
	// (controller internal state, do not use)
	// +optional
	Hash string `json:"Hash,omitempty"`

	// TFState is the Terraform state.
	// (controller internal state, do not use)
	// +optional
	TFState string `json:"zTFState,omitempty"`
}

type ClusterStatus struct {
	// Result values provided to access a cluster.
	CA     string          `json:"certificate-authority-data,omitempty"`
	Server string          `json:"server,omitempty"`
	User   UserCredentials `json:"user,omitempty"`

	// Hash is an unique value for the (addons) source and parameters being deployed.
	// Hash includes infra.hash to express the dependency of clusters on infra.
	// (controller internal state, do not use)
	// +optional
	Hash string `json:"hash,omitempty"`
}

// UserCredentials are the result values to authenticate with a cluster.
type UserCredentials struct {
	ClientCertificate string `json:"string client-certificate,omitempty"`
	ClientKey         string `json:"client-key,omitempty"`
	Password          string `json:"password,omitempty"`
	Username          string `json:"username,omitempty"`
	//TODO needed? AOSHA             string `json:"aoSHA,omitempty"`
}

/*TODO remove
// EnvironmentCondition
type EnvironmentCondition struct {
	// Type is the name of the condition.
	Type EnvironmentConditionType `json:"type,omitempty"`

	// Status of the condition, one of True, False, Unknown.
	Status metav1.ConditionStatus `json:"status,omitempty"`

	// Last time the condition status has changed.
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`

	// Reason for last transition in a single word.
	// +optional
	Reason string `json:"reason,omitempty"`

	// Human readable message indicating details about last transition.
	// Mandatory when Status goes False.
	// +optional
	Message string `json:"message,omitempty"`
}

type EnvironmentConditionType string
const (
	EnvironmentIniting EnvironmentConditionType = "initing"
	EnvironmentInited EnvironmentConditionType = "inited"
	EnvironmentPlanning EnvironmentConditionType = "planning"
	EnvironmentPlanned EnvironmentConditionType = "planned"
	EnvironmentApplying EnvironmentConditionType = "applying"
	EnvironmentApplied EnvironmentConditionType = "applied"
)*/

// EnvironmentCondition shows what the operator is doing or has done.
// Infra condition types: InfraTmplt, InfraInit, InfraPlan, InfraApply
// Cluster condition types: ClusterAddon<ClusterName> ClusterTest<ClusterName>
// See KEP sig-api-machinery/1623-standardize-conditions is going to introduce it as k8s.io/apimachinery/pkg/apis/meta/v1
type EnvironmentCondition struct {
	// Type of condition in CamelCase.
	// +required
	Type string `json:"type" protobuf:"bytes,1,opt,name=type"`
	// Status of the condition, one of True, False, Unknown.
	// +required
	Status metav1.ConditionStatus `json:"status" protobuf:"bytes,2,opt,name=status"`
	// Last time the condition transitioned from one status to another.
	// This should be when the underlying condition changed.  If that is not known, then using the time when the API field changed is acceptable.
	// +required
	LastTransitionTime metav1.Time `json:"lastTransitionTime" protobuf:"bytes,3,opt,name=lastTransitionTime"`
	// The reason for the condition's last transition in CamelCase.
	// +required
	Reason EnvironmentConditionReason `json:"reason" protobuf:"bytes,4,opt,name=reason"`
	// A human readable message indicating details about the transition.
	// +optional
	Message string `json:"message,omitempty" protobuf:"bytes,5,opt,name=message"`
}

// EnvironmentConditionReason is the reason for the condition change.
type EnvironmentConditionReason string

const (
	ReasonRunning EnvironmentConditionReason = "Running"
	ReasonReady   EnvironmentConditionReason = "Ready"
	ReasonFailed  EnvironmentConditionReason = "Failed"
)

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Synced",type=string,JSONPath=`.status.synced`
// +kubebuilder:subresource:status

// Environment is the Schema for the environments API
type Environment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EnvironmentSpec   `json:"spec,omitempty"`
	Status EnvironmentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// EnvironmentList contains a list of Environment
type EnvironmentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Environment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Environment{}, &EnvironmentList{})
}
