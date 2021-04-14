// packe az provides a simple wrapper around the Azure CLI.
package azure

import (
	"fmt"
	"github.com/go-logr/logr"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/util/exe"
	"strings"
)

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
	// Autoscaling enables or disables a Node autoscaler.
	Autoscaler(enable bool, resourceGroup string, cluster string, pool string, minCount int, maxCount int) error
	// AllAutoscalers enables or disables the Node autoscalers of multiple clusters.
	AllAutoscalers(enable bool, clusters []v1.ClusterSpec, resourceGroup string, log logr.Logger) error
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

// RunAZ runs the az cli.
func runAZ(log logr.Logger, options *exe.Opt, stdin string, args ...string) (string, error) {
	stdout, stderr, err := exe.Run(log, options, stdin, "az", args...)
	if err != nil {
		return "", err
	}
	// don't rely on az cli exit code for error detection.
	if stderr != "" {
		return "", fmt.Errorf("%s: %s", strings.Join(args, " "), stderr)
	}
	return stdout, err
}
