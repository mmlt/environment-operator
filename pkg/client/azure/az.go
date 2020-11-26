// packe az provides a simple wrapper around the Azure CLI.
package azure

import "github.com/go-logr/logr"

// AZer is able to perform az cli commands.
type AZer interface {
	SetSubscription(sub string)

	LoginSP(user, password, tenant string) error
	Logout() error

	// KeyvaultSecret reads name secret from vaultName KeyVault.
	KeyvaultSecret(name, vaultName string) (string, error)

	// AKSNodepoolList returns all the node pools of an AKS cluster.
	AKSNodepoolList(resourceGroup, cluster string) ([]AKSNodepool, error)
	// AKSNodepool returns the details about an AKS cluster nodepool.
	AKSNodepool(resourceGroup, cluster, nodepool string) (*AKSNodepool, error)
	// AKSNodepoolUpgrade upgrades the node pool in a managed Kubernetes cluster to Kubernetes version.
	// Expect this call to block for VM count * 10m.
	AKSNodepoolUpgrade(resourceGroup, cluster, nodepool, version string) (*AKSNodepool, error)
}

// AZ is able to perform az cli commands.
type AZ struct {
	// Subscription is the Name or ID of the Azure subscription.
	Subscription string

	Log logr.Logger
}

// SetSubscription sets the Name or ID of the Azure subscription to use.
func (c *AZ) SetSubscription(sub string) {
	c.Subscription = sub
}

// ExtraArgs appends global arguments to arg and returns the result.
func (c *AZ) extraArgs(arg []string) []string {
	if c.Subscription != "" {
		arg = append(arg, "--subscription", c.Subscription)
	}
	return arg
}
