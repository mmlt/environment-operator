package azure

import (
	"fmt"
	"github.com/go-logr/logr"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"strconv"
	"strings"
)

// Autoscaler enables or disables a Node autoscaler.
// https://docs.microsoft.com/en-us/azure/aks/cluster-autoscaler#use-the-cluster-autoscaler-with-multiple-node-pools-enabled
func (c *AZ) Autoscaler(enable bool, cluster string, pool AKSNodepool) error {
	args := []string{"aks", "nodepool", "update", "--resource-group", pool.ResourceGroup, "--cluster-name", cluster,
		"--name", pool.Name}
	if enable {
		args = append(args,
			"--min-count", strconv.Itoa(pool.MinCount),
			"--max-count", strconv.Itoa(pool.MaxCount),
			"--enable-cluster-autoscaler")
	} else {
		args = append(args, "--disable-cluster-autoscaler")
	}

	_, err := runAZ(c.Log, nil, "", args...)

	return err
}

// AllAutoscalers enables or disables the Node autoscalers of multiple clusters.
func (c *AZ) AllAutoscalers(enable bool, clusters []v1.ClusterSpec, resourceGroup string, log logr.Logger) error {
	for _, cl := range clusters {
		pls, err := c.AKSNodepoolList(resourceGroup, cl.Name)
		if err != nil {
			s := fmt.Sprintf("ERROR: The Resource 'Microsoft.ContainerService/managedClusters/%s' under resource group '%s' was not found", cl.Name, resourceGroup)
			if strings.Contains(err.Error(), s) {
				log.Info("ignore error", "error", err.Error())
				break
			}
			return err
		}
		for _, pl := range pls {
			if pl.EnableAutoScaling {
				err = c.Autoscaler(enable, cl.Name, pl)
				log.Error(err, "disable autoscaler", "cluster", cl.Name, "pool", pl.Name)
			}
		}
	}
	return nil
}
