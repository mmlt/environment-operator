package azure

import (
	"time"
)

// TerraformFake provides a Terraformer for testing.
type AZFake struct {
	// Tally is the number of times a method has been called.
	AKSNodepoolListTally, AKSNodepoolTally, AKSNodepoolUpgradeTally int

	// Results that are returned by the fake implementations.
	AKSNodepoolListResult []AKSNodepool
	// AKSNodepoolResult is a list of results of which one is returned on each subsequent call.
	AKSNodepoolResult        []AKSNodepool
	AKSNodepoolUpgradeResult AKSNodepool
}

// AKSNodepoolList returns all the node pools of an AKS cluster.
func (c *AZFake) AKSNodepoolList(resourceGroup, cluster string) ([]AKSNodepool, error) {
	c.AKSNodepoolListTally++
	return c.AKSNodepoolListResult, nil
}

// AKSNodepool returns the details about an AKS cluster nodepool.
func (c *AZFake) AKSNodepool(resourceGroup, cluster, nodepool string) (*AKSNodepool, error) {
	c.AKSNodepoolTally++
	i := c.AKSNodepoolTally
	if i > len(c.AKSNodepoolResult) {
		i = len(c.AKSNodepoolResult)
	}
	r := c.AKSNodepoolUpgradeResult
	r.ProvisioningState = c.AKSNodepoolResult[i-1].ProvisioningState
	return &r, nil
}

// AKSNodepoolUpgrade upgrades the node pool in a managed Kubernetes cluster to Kubernetes version.
// Expect this call to block for 10m per VM.
func (c *AZFake) AKSNodepoolUpgrade(resourceGroup, cluster, nodepool, version string) (*AKSNodepool, error) {
	c.AKSNodepoolUpgradeTally++
	time.Sleep(15 * time.Second)
	return &c.AKSNodepoolUpgradeResult, nil
}

// SetupFakeResults sets-up the receiver with data that is returned during testing.
func (c *AZFake) SetupFakeResults() {
	c.AKSNodepoolListResult = []AKSNodepool{
		{
			AgentPoolType:       "VirtualMachineScaleSets",
			Count:               2,
			Mode:                "User",
			Name:                "default",
			OrchestratorVersion: "1.16.0", // results in upgrade
			OSType:              "Linux",
			ProvisioningState:   "Succeeded",
			VMSize:              "Standard_DS2_v2",
		},
		{
			AgentPoolType:       "VirtualMachineScaleSets",
			Count:               5,
			Mode:                "User",
			Name:                "extra",
			OrchestratorVersion: "1.16.0",
			OSType:              "Linux",
			ProvisioningState:   "Succeeded",
			VMSize:              "Standard_DS2_v2",
		},
	}

	c.AKSNodepoolResult = []AKSNodepool{
		{ProvisioningState: "Creating"},
		{ProvisioningState: "Succeeded"},
	}

	c.AKSNodepoolUpgradeResult = AKSNodepool{
		AgentPoolType:       "VirtualMachineScaleSets",
		Count:               2,
		Mode:                "User",
		Name:                "default",
		OrchestratorVersion: "1.16.8",
		OSType:              "Linux",
		ProvisioningState:   "Succeeded",
		VMSize:              "Standard_DS2_v2",
	}
}
