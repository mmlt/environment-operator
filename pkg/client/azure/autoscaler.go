package azure

import (
	"strconv"
)

// Autoscaler enables or disables the autoscaler.
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
