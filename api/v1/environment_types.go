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
	// Destroy is true when an environment needs to be removed.
	// Typically used in cluster delete/create test cases.
	// (in addition to destroy: true a budget.deleteLimit: 99 is required)
	Destroy bool `json:"destroy,omitempty"`

	// Infra defines infrastructure that much exist before clusters can be created.
	Infra InfraSpec `json:"infra,omitempty"`

	// Defaults defines the values common to all Clusters.
	Defaults ClusterSpec `json:"defaults,omitempty"`

	// Clusters defines the values specific for each cluster instance.
	Clusters []ClusterSpec `json:"clusters,omitempty"`
}

// InfraSpec defines the infrastructure that is used by all clusters.
type InfraSpec struct {
	// EnvName is the name of this environment.
	// Typically a concatenation of region, cloud provider and environment type (test, production).
	EnvName string `json:"envName,omitempty"`

	// EnvDomain is the most significant part of the domain name for this environment.
	// For example; example.com
	EnvDomain string `json:"envDomain,omitempty"`

	// Budget defines how many changes the operator is allowed to apply to the infra.
	// If the budget spec is omitted any number of changes is allowed.
	// +optional
	Budget InfraBudget `json:"budget,omitempty"`

	// Schedule is a CRON formatted string defining when changed can be applied.
	// If the schedule is omitted then changes will be applied immediately.
	// +optional
	Schedule string `json:"schedule,omitempty"`

	// Source is the repository that contains Terraform infrastructure code.
	Source SourceSpec `json:"source,omitempty"`

	// Main is the path in the source tree to the directory containing main.tf.
	Main string `json:"main,omitempty"`

	// State is where Terraform state is stored.
	// If the state spec is omitted the state is stored locally.
	// +optional
	State StateSpec `json:"state,omitempty"`

	// AAD is the Azure Active Directory that is queried when a k8s user authorization is checked.
	AAD AADSpec `json:"aad,omitempty"`

	// AZ contains Azure specific values.
	// +optional
	AZ AZSpec `json:"az,omitempty"`

	// X are extension values (when regular values don't fit the need)
	// +optional
	X map[string]string `json:"x,omitempty"`
}

// InfraBudget defines how many changes the operator is allowed to make.
type InfraBudget struct {
	// AddLimit is the maximum number of resources that the operator is allowed to add.
	// Exceeded this number will result in an error.
	// +optional
	AddLimit *int32 `json:"addLimit,omitempty"`

	// UpdateLimit is the maximum number of resources that the operator is allowed to update.
	// Exceeded this number will result in an error.
	// +optional
	UpdateLimit *int32 `json:"updateLimit,omitempty"`

	// DeleteLimit is the maximum number of resources that the operator is allowed to delete.
	// Exceeded this number will result in an error.
	// +optional
	DeleteLimit *int32 `json:"deleteLimit,omitempty"`
}

// ClusterSpec defines cluster specific infra and k8s resources.
type ClusterSpec struct {
	// Name is the cluster name.
	Name string `json:"name,omitempty"`

	// Infra defines cluster specific infrastructure.
	Infra ClusterInfraSpec `json:"infra,omitempty"`

	// ClusterAddonSpec defines the Kubernetes resources to deploy to have a functioning cluster.
	Addons ClusterAddonSpec `json:"addons,omitempty"`
}

// SourceSpec defines the location to fetch content like configuration scripts and tests from.
type SourceSpec struct {
	// Type is the type of repository to use as a source.
	// Valid values are:
	// - "git" (default): GIT repository.
	// - "local": local filesystem.
	// +optional
	Type EnvironmentSourceType `json:"type,omitempty"`

	// For type=git URL is the URL of the repo.
	// When Token is specified the URL is expected to start with 'https://'.
	//
	// For type=local URL is path to a directory.
	// +optional
	URL string `json:"url"`

	// Ref is the reference to the content to get.
	// For type=git it can be 'master', 'refs/heads/my-branch' etc, see 'git reference' doc.
	// For type=local the value can be omitted.
	// +optional
	Ref string `json:"ref,omitempty"`

	// Token is used to authenticate with the remote server (only applicable when Type=git)
	// Instead of a token a reference in the form "vault name field" o token can be used.
	// An alternative authentication method is to have an SSH key present in ~/.ssh.
	// +optional
	Token string `json:"token,omitempty"`

	// Area is a directory path to the part of the repo that contains the required contents.
	// Typically area is empty indicating that the whole repo is used.
	// When only part of the repo is used and changes to other parts of the repo should be ignored let point area to
	// that relevant part.
	// +optional
	Area string `json:"area,omitempty"`
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

// StateSpec specifies where to find the Terraform state storage.
// +optional
type StateSpec struct {
	// StorageAccount is the name of the Storage Account.
	StorageAccount string `json:"storageAccount,omitempty"`
	// Access is the secret that allows access to the storage account
	// or a reference to that secret in the form "vault secret-name field-name"
	Access string `json:"access,omitempty"`
}

// Azure Active Directory.
type AADSpec struct {
	// TenantID is the AD tenant or a reference to that value in the form "vault name field"
	TenantID string `json:"tenantID,omitempty"`
	// ServerAppID is an app registration allowed to query AD for user data
	// or a reference to that value in the form "vault name field"
	ServerAppID string `json:"serverAppID,omitempty"`
	// ServerAppSecret is the secret of an app registration allowed to query AD for user data
	// or a reference to that value in the form "vault name field"
	ServerAppSecret string `json:"serverAppSecret,omitempty"`
	// ClientAppID is the app registration used by kubectl or a reference to that value in the form "vault name field"
	ClientAppID string `json:"clientAppID,omitempty"`
}

// AZSpec defines Azure specific infra structure settings.
type AZSpec struct {
	// Subscription is a list of one or more subscriptions used during provisioning.
	// The first subscription is the default subscription.
	Subscription []AZSubscription `json:"subscription,omitempty"`

	// ResourceGroup
	ResourceGroup string `json:"resourceGroup,omitempty"`

	// VNet CIDR is the network range used by one or more clusters.
	VNetCIDR string `json:"vnetCIDR,omitempty"`

	// DNS is an optional list of custom DNS servers.
	// (VM's in VNet need to be restarted to propagate changes to this value)
	// +optional
	DNS []string `json:"dns,omitempty"`

	// Subnet newbits is the number of bits to add to the VNet address mask to produce the subnet mask.
	// IOW 2^subnetNewbits-1 is the max number of clusters in the VNet.
	// For example given a /16 VNetCIDR and subnetNewbits=4 would result in /20 subnets.
	SubnetNewbits int32 `json:"subnetNewbits,omitempty"`

	// Outbound sets the network outbound type.
	// Valid values are:
	// - loadBalancer (default)
	// - userDefinedRouting
	// +kubebuilder:validation:Enum=loadBalancer;userDefinedRouting
	Outbound AZOutbound `json:"outbound,omitempty"`

	// Routes is an optional list of routes
	// +optional
	Routes []AZRoute `json:"routes,omitempty"`
}

// AZSubscription is an Azure Subscription.
type AZSubscription struct {
	// Name of the subscription.
	Name string `json:"name,omitempty"`

	// ID of the subscription.
	ID string `json:"id,omitempty"`
}

// AZOutbound sets the network outbound type.
type AZOutbound string

const (
	OutboundLoadbalancer     AZOutbound = "loadBalancer"
	OutboundUserDefinedRoute AZOutbound = "userDefinedRoute"
)

// AZSKU sets the SLA on the AKS control plane.
type AZSKU string

const (
	SKUFree AZSKU = "Free"
	SKUPaid AZSKU = "Paid"
)

// +kubebuilder:validation:Enum=Microsoft.AzureActiveDirectory;Microsoft.AzureCosmosDB;Microsoft.ContainerRegistry;Microsoft.EventHub;Microsoft.KeyVault;Microsoft.ServiceBus;Microsoft.Sql;Microsoft.Storage;Microsoft.Web
type AZServiceEndpoint string

// AZLogAnalyticsWorkspace defines a sink for Kubernetes control plane log data.
type LogAnalyticsWorkspace struct {
	// SubscriptionName of the Log Analytics workspace.
	// This name refers to infra.az.subscription list of subscriptions.
	SubscriptionName string `json:"subscriptionName,omitempty"`
	// ResourceGroup name of the Log Analytics workspace.
	ResourceGroupName string `json:"resourceGroupName,omitempty"`
	// Name of the Log Analytics workspace.
	Name string `json:"name,omitempty"`
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
	SubnetNum int32 `json:"subnetNum,omitempty"`

	// Kubernetes version.
	Version string `json:"version,omitempty"`

	// Cluster worker pools.
	// NB. For AKS a pool named 'default' must be defined.
	Pools map[string]NodepoolSpec `json:"pools,omitempty"`

	// AZ contains Azure specific values for clusters.
	// +optional
	AZ ClusterAZSpec `json:"az,omitempty"`

	// X are extension values (when regular values don't fit the need)
	// +optional
	X map[string]string `json:"x,omitempty"`
}

// NodepoolSpec defines a cluster worker node pool.
type NodepoolSpec struct {
	// Mode selects the purpose of a pool; User (default) or System.
	// AKS doc https://docs.microsoft.com/en-us/azure/aks/use-system-pools
	// +optional
	// +kubebuilder:validation:Enum=System;User
	Mode string `json:"mode,omitempty"`

	// An optional map of Kubernetes node labels.
	// Changing this forces a new resource to be created.
	// +optional
	NodeLabels map[string]string `json:"nodeLabels,omitempty"`

	// An optional list of Kubernetes node taints (e.g CriticalAddonsOnly=true:NoSchedule).
	// Changing this forces a new resource to be created.
	// +optional
	NodeTaints []string `json:"nodeTaints,omitempty"`

	// Type of VM's.
	// Changing this forces a new resource to be created.
	VMSize string `json:"vmSize,omitempty"`

	// Max number of Pods per VM.
	// Changing this forces a new resource to be created.
	// +kubebuilder:validation:Minimum=10
	// +kubebuilder:validation:Maximum=250
	MaxPods int32 `json:"maxPods,omitempty"`

	// Number of VM's.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	Scale int32 `json:"scale,omitempty"`

	// Max number of VM's.
	// Setting MaxScale > Scale enables autoscaling.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	MaxScale int32 `json:"maxScale,omitempty"`
}

// ClusterAZSpec contains Azure specific values for clusters.
type ClusterAZSpec struct {
	// AvailabilityZones are the zones in a region over which nodes and control plane are spread.
	// For example [1,2,3]
	// +optional
	AvailabilityZones []int32 `json:"availabilityZones,omitempty"`

	// SKU (stock keeping unit) sets the SLA on the AKS control plane.
	// Valid values are:
	// - Free (default)
	// - Paid
	// +optional
	// +kubebuilder:validation:Enum=Free;Paid
	SKU AZSKU `json:"sku,omitempty"`

	// ServiceEndpoints provide direct connectivity to Azure services over the Azure backbone network.
	// This is an optional list of one or more of the following values:
	// Microsoft.AzureActiveDirectory, Microsoft.AzureCosmosDB, Microsoft.ContainerRegistry, Microsoft.EventHub,
	// Microsoft.KeyVault, Microsoft.ServiceBus, Microsoft.Sql, Microsoft.Storage and Microsoft.Web
	// +optional
	ServiceEndpoints []AZServiceEndpoint `json:"serviceEndpoints,omitempty"`

	// LogAnalyticsWorkspace is a sink for Kubernetes control plane log data.
	// This is an optional value.
	// +optional
	LogAnalyticsWorkspace *LogAnalyticsWorkspace `json:"logAnalyticsWorkspace,omitempty"`
}

// ClusterAddonSpec defines what K8s resources needs to be deployed in a cluster after creation.
type ClusterAddonSpec struct {
	// Schedule is a CRON formatted string defining when changed can be applied.
	// If the schedule is omitted then changes will be applied immediately.
	// +optional
	Schedule string `json:"schedule,omitempty"`

	// Source is the repository that contains the k8s addons resources.
	Source SourceSpec `json:"source,omitempty"`

	// Jobs is an array of paths to job files in the source tree.
	Jobs []string `json:"jobs,omitempty"`

	// MKV is the path to a directory in the source tree that specifies the master key vault to use.
	MKV string `json:"mkv,omitempty"`

	// X are extension values (when regular values don't fit the need)
	// +optional
	X map[string]string `json:"x,omitempty"`
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
	// An opaque value representing the config/parameters applied by a step.
	// Only valid when state=Ready.
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

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].reason"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message"

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
