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

	// Destroy is true when an environment needs to be removed.
	// Typically used in cluster delete/create test cases.
	Destroy bool `json:"destroy,omitempty"`

	// Infra defines infrastructure that much exist before clusters can be created.
	Infra InfraSpec `json:"infra,omitempty"`

	// Defaults defines the values common to all Clusters.
	Defaults ClusterSpec `json:"defaults,omitempty"`

	// Clusters defines the values specific for each cluster instance.
	Clusters []ClusterSpec `json:"clusters,omitempty"`

	//TODO implement Tests
}

// EnvironmentPolicy describes how the environment will be updated or deleted.
// Only one of the following policies may be specified.
// If none is specified, the default one AllowAll.
// +kubebuilder:validation:Enum=AllowAll;DenyDelete;DenyUpdate
type EnvironmentPolicy string

const (
	// AllowAll allows create, update and delete of cluster add-ons.
	PolicyAllowAll EnvironmentPolicy = "AllowAll"

	// DenyDelete forbids delete of cluster add-ons when ClusterAddon resource is deleted.
	PolicyDenyDelete EnvironmentPolicy = "DenyDelete"

	// DenyUpdate forbids update/delete of cluster add-ons when ClusterAddon or repo changes.
	PolicyDenyUpdate EnvironmentPolicy = "DenyUpdate"
)

// InfraSpec defines the infrastructure that the clusters depend on.
type InfraSpec struct {
	// Source is the repository that contains Terraform infrastructure code.
	Source SourceSpec `json:"source,omitempty"`

	// Main is the path in the source tree to main.tf.
	Main string `json:"main,omitempty"`

	// EnvDomain is the most significant part of the domain name for this environment.
	// For example; example.com
	EnvDomain string `json:"envDomain,omitempty"`

	// EnvName is the name of this environment.
	// Typically a concatenation of region, cloud provider and environment type (test, production).
	EnvName string `json:"envName,omitempty"`

	// AAD is the Azure Active Directory that is queried when a k8s user authorization is checked.
	AAD AADSpec `json:"aad,omitempty"`

	// AZ values.
	// +optional
	AZ AZSpec `json:"az,omitempty"`
}

// ClusterSpec defines cluster specific infra, K8s resources and tests.
type ClusterSpec struct {
	// Name is the cluster name.
	Name string `json:"name,omitempty"`

	// Infra defines cluster specific infrastructure.
	Infra ClusterInfraSpec `json:"infra,omitempty"`

	// ClusterAddonSpec defines the Kubernetes resources to deploy to have a functioning cluster.
	Addons ClusterAddonSpec `json:"addons,omitempty"`

	// ClusterAddonSpec defines what conformance test to run.
	Test ClusterTestSpec `json:"test,omitempty"`
}

// SourceSpec defines the location to fetch content like configuration scripts and tests from.
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

// AZSpec defines Azure specific infra structure settings.
type AZSpec struct {
	// Subscription
	Subscription string `json:"subscription,omitempty"`

	// ResourceGroup
	ResourceGroup string `json:"resourceGroup,omitempty"`

	// The VNet CIDR that connects one or more clusters.
	VNetCIDR string `json:"vnetCIDR,omitempty"`

	// DNS is an optional list of custom DNS servers.
	// (VM's in VNet need to be restarted to propagate changes to this value)
	// +optional
	DNS []string `json:"dns,omitempty"`

	// Subnet newbits is the number of bits to add to the VNet address mask to produce the subnet mask.
	// IOW 2^subnetNewbits-1 is the max number of clusters in the VNet.
	// For example given a /16 VNetCIDR and subnetNewbits=4 would result in /20 subnets.
	SubnetNewbits int `json:"subnetNewbits,omitempty"`

	// Routes is an optional list of routes
	// +optional
	Routes []AZRoute `json:"routes,omitempty"`

	// X are extension values (when regular values don't fit the need)
	// +optional
	X map[string]string `json:"x,omitempty"`
}

// AZRoute is an entry in the routing table of the VNet.
type AZRoute struct {
	Name               string `json:"name,omitempty" hcl:"name"`
	AddressPrefix      string `json:"addressPrefix,omitempty" hcl:"address_prefix"`
	NextHopType        string `json:"nextHopType,omitempty" hcl:"next_hop_type"`
	NextHopInIPAddress string `json:"nextHopInIPAddress,omitempty" hcl:"next_hop_in_ip_address" hcle:"omitempty"`
}

// ClusterInfraSpec defines cluster specific infrastructure.
type ClusterInfraSpec struct {
	// Cluster ordinal number starting at 1.
	// (max 2^subnetNewbits-1)
	// +kubebuilder:validation:Minimum=1
	SubnetNum int `json:"subnetNum,omitempty"`

	// Kubernetes version.
	Version string `json:"version,omitempty"`

	// Cluster worker pools.
	// NB. For AKS a pool named 'default' must be defined.
	Pools map[string]NodepoolSpec `json:"pools,omitempty"`

	// X are extension values (when regular values don't fit the need)
	// +optional
	X map[string]string `json:"x,omitempty"`
}

// NodepoolSpec defines a cluster worker node pool.
type NodepoolSpec struct {
	// Number of VM's.
	Scale int `json:"scale,omitempty"`

	// Type of VM's.
	VMSize string `json:"vmSize,omitempty"`
}

// ClusterAddonSpec defines what K8s resources needs to be deployed in a cluster after creation.
type ClusterAddonSpec struct {
	// Source is the repository that contains the k8s addons resources.
	Source SourceSpec `json:"source,omitempty"`

	// Jobs is an array of paths to job files in the source tree.
	Jobs []string `json:"jobs,omitempty"`

	// X are extension values (when regular values don't fit the need)
	// +optional
	X map[string]string `json:"x,omitempty"`
}

type ClusterTestSpec struct {
	//TODO implement ClusterTestSpec
}

// EnvironmentStatus defines the observed state of an Environment.
type EnvironmentStatus struct {
	// Conditions are a synopsis of the StepStates.
	// +optional
	Conditions []EnvironmentCondition `json:"conditions,omitempty"`

	// Step contains the latest available observations of the Environment's state.
	Steps map[string]StepStatus `json:"steps,omitempty"`
}

// StepStatus is the last observed status of a Step.
type StepStatus struct {
	// Last time the state transitioned.
	// This should be when the underlying condition changed.  If that is not known, then using the time when the API field changed is acceptable.
	// +required
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
	// The reason for the StepState's last transition in CamelCase.
	// +required
	State StepState `json:"state"`
	// A human readable message indicating details about the transition.
	// +optional
	Message string `json:"message,omitempty"`
	// An opaque value representing the config/parameters applied by the step.
	Hash string `json:"hash,omitempty"`
}

// StepState is the current state of the step.
type StepState string

const (
	StateRunning StepState = "Running"
	StateReady   StepState = "Ready"
	StateError   StepState = "Error"
)

// EnvironmentCondition provides a synopsis of the current environment state.
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

// Environment is an environment at a cloud-provider with one or more Kubernetes clusters, addons, conformance tested.
type Environment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EnvironmentSpec   `json:"spec,omitempty"`
	Status EnvironmentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// EnvironmentList contains a list of Environments.
type EnvironmentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Environment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Environment{}, &EnvironmentList{})
}
