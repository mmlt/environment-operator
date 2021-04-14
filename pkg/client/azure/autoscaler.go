package azure

import (
	"fmt"
	"github.com/go-logr/logr"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"strconv"
	"strings"
)

// Autoscaler enables or disables a Node autoscaler for a cluster/pool.
// https://docs.microsoft.com/en-us/azure/aks/cluster-autoscaler#use-the-cluster-autoscaler-with-multiple-node-pools-enabled
func (c *AZ) Autoscaler(enable bool, resourceGroup string, cluster string, pool string, minCount int, maxCount int) error {
	var a string

	args := []string{"aks", "nodepool", "update", "--resource-group", resourceGroup, "--cluster-name", cluster,
		"--name", pool}
	if enable {
		args = append(args,
			"--min-count", strconv.Itoa(minCount),
			"--max-count", strconv.Itoa(maxCount),
			"--enable-cluster-autoscaler")
		a = "enable"
	} else {
		args = append(args, "--disable-cluster-autoscaler")
		a = "disable"
	}

	c.Log.Info(a+" autoscaler", "resourceGroup", resourceGroup, "cluster", cluster, "pool", pool)

	_, err := runAZ(c.Log, nil, "", args...)

	return err
}

// AllAutoscalers enables or disables Node autoscaling of multiple clusters/pools.
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
				err = c.Autoscaler(enable, pl.ResourceGroup, cl.Name, pl.Name, pl.MinCount, pl.MaxCount)
				log.Error(err, "disable autoscaler", "cluster", cl.Name, "pool", pl.Name)
			}
		}
	}
	return nil
}
