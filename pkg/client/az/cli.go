// Package az provides a simple wrapper around az cli.
package az

import (
	"encoding/json"
	"github.com/go-logr/logr"
	"github.com/mmlt/environment-operator/pkg/util/exe"
)

// CLI provides access to Azure resources via az cli.
type CLI struct {
	ResourceGroup string
	Log           logr.Logger
}

// New creates a local environment at dir to Get data from a GIT repo at url/reference.
// Argument reference can be master, refs/heads/my-branch etc, see https://git-scm.com/docs/git-show-ref
func New(resourceGroup string, log logr.Logger) (*CLI, error) {
	return &CLI{
		ResourceGroup: resourceGroup,
		Log:           log,
	}, nil
}

// https://docs.microsoft.com/en-us/cli/azure/ext/aks-preview/aks/nodepool

// AKSNodepoolList returns all the node pools of an AKS cluster.
func (c *CLI) AKSNodepoolList(cluster string) ([]AKSNodepool, error) {
	args := []string{"aks", "nodepool", "list", "--resource-group", c.ResourceGroup, "--cluster-name", cluster}
	o, _, err := exe.Run(c.Log, nil, "", "az", args...)
	if err != nil {
		return nil, err
	}

	var r []AKSNodepool
	err = json.Unmarshal([]byte(o), &r)
	if err != nil {
		return nil, err
	}

	return r, nil
}

// AKSNodepool returns the details an AKS cluster nodepool.
func (c *CLI) AKSNodepool(cluster, nodepool string) (*AKSNodepool, error) {
	args := []string{"aks", "nodepool", "show", "--resource-group", c.ResourceGroup, "--cluster-name", cluster,
		"--name", nodepool}
	o, _, err := exe.Run(c.Log, nil, "", "az", args...)
	if err != nil {
		return nil, err
	}

	var r AKSNodepool
	err = json.Unmarshal([]byte(o), &r)
	if err != nil {
		return nil, err
	}

	return &r, nil
}

// AKSNodepoolUpgrade upgrades the node pool in a managed Kubernetes cluster to Kubernetes version.
// Expect this call to block for VM count * 10m.
func (c *CLI) AKSNodepoolUpgrade(cluster, nodepool, version string) (*AKSNodepool, error) {
	args := []string{"aks", "nodepool", "upgrade", "--resource-group", c.ResourceGroup, "--cluster-name", cluster,
		"--name", nodepool, "--kubernetes-version", version}
	o, _, err := exe.Run(c.Log, nil, "", "az", args...)
	if err != nil {
		return nil, err
	}

	var r AKSNodepool
	err = json.Unmarshal([]byte(o), &r)
	if err != nil {
		return nil, err
	}

	return &r, nil
}

// AKSNodepool is a subset of AKS node pool values.
type AKSNodepool struct {
	// AgentPoolType is VirtualMachineScaleSets or AvailabilitySet
	AgentPoolType string `json:"agentPoolType"`
	// Count is the number of VM's in the pool.
	Count int `json:"count"`
	// Mode defines the primary function of the pool.
	// If set as "System", AKS prefers system pods scheduling to the pool. https://aka.ms/aks/nodepool/mode.
	Mode string `json:"mode"`
	// Name of the pool.
	Name string `json:"name"`
	// OrchestratorVersion is the Kuberntes version of the pool.
	OrchestratorVersion string `json:"orchestratorVersion"`
	// AgentPoolType is the OS Type; Linux or Windows.
	OSType string `json:"osType"`
	// ProvisioningState is the current state of the pool.
	ProvisioningState ProvisioningState `json:"provisioningState"`
	// VMSize is the type of VM used in the pool.
	VMSize string `json:"vmSize"`
}

// ProvisioningState represents the current state of container service resource.
// https://github.com/Azure/aks-engine/blob/master/pkg/api/agentPoolOnlyApi/vlabs/types.go
type ProvisioningState string

const (
	// Creating means ContainerService resource is being created.
	Creating ProvisioningState = "Creating"
	// Updating means an existing ContainerService resource is being updated
	Updating ProvisioningState = "Updating"
	// Failed means resource is in failed state
	Failed ProvisioningState = "Failed"
	// Succeeded means resource created succeeded during last create/update
	Succeeded ProvisioningState = "Succeeded"
	// Deleting means resource is in the process of being deleted
	Deleting ProvisioningState = "Deleting"
	// Migrating means resource is being migrated from one subscription or
	// resource group to another
	Migrating ProvisioningState = "Migrating"
)
