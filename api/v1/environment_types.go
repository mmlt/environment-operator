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

	// Values are key-value pairs that are passed as tfvars to Terraform.
	// +optional
	Values map[string]string `json:"values,omitempty"`
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

	// +kubebuilder:validation:MinLength=2

	// URL is the URL of the repo.
	// When Token is specified the URL is expected to start with 'https://'.
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

type AddonSpec struct {
	// Source is the repository that contains Terraform infrastructure code.
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
	// Peek is a single word describing the Environment's fitness.
	// Check conditions for more details.
	// +optional
	Peek EnvironmentPeekType `json:"peek,omitempty"`

	// Conditions are the latest available observations of the Environment's fitness.
	// +optional
	Conditions []EnvironmentCondition `json:"conditions,omitempty"`

	// Clusters contains the coordinates of the clusters.
	// +optional
	Clusters map[string]ClusterStatus `json:"clusters,omitempty"`

	// The following fields contain state that's used by the controller.

	// TFSHA is the source repo SHA of the last successfully applied Terraform code.
	// (controller internal state, do not use)
	// +optional
	TFSHA string `json:"tfSHA,omitempty"`

	// TFState is the Terraform state.
	// (controller internal state, do not use)
	// +optional
	TFState string `json:"tfState,omitempty"`
}

// EnvironmentPeekType is the one word condition of the Environment.
type EnvironmentPeekType string

const (
	// PeekUnknown means the Environment state is unknown.
	PeekUnknown EnvironmentPeekType = ""
	// PeekReady means the Enviroment is in the desired state and there is nothing to do.
	PeekReady EnvironmentPeekType = "Ready"
)

type ClusterStatus struct {
	CA     string     `json:"certificate-authority-data,omitempty"`
	Server string     `json:"server,omitempty"`
	User   UserStatus `json:"user,omitempty"`
}

type UserStatus struct {
	ClientCertificate string `json:"string client-certificate,omitempty"`
	ClientKey         string `json:"client-key,omitempty"`
	Password          string `json:"password,omitempty"`
	Username          string `json:"username,omitempty"`
	AOSHA             string `json:"aoSHA,omitempty"`
}

type EnvironmentCondition struct {
	//TODO implement Conditions
}

// +kubebuilder:object:root=true

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
