package step

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/client/azure"
	"sort"
	"time"
)

// AKSPoolStep can upgrade AKS node pools to the desired Kubernetes version.
type AKSPoolStep struct {
	Metaa

	/* Parameters */

	// ResourceGroup that contains Cluster.
	ResourceGroup string
	// Cluster is the name of the AKS cluster to upgrade the node pool(s) of.
	// NB. This is the AKS name (which is the short name with a prefix).
	Cluster string
	// Version is the Kubernetes version to upgrade the node pool(s) to.
	Version string

	// Azure is the azure cli implementation to use.
	Azure azure.AZer
}

// Execute node pool upgrade for a cluster.
func (st *AKSPoolStep) Execute(ctx context.Context, _ []string) {
	log := logr.FromContext(ctx).WithName("AKSPoolStep").WithValues("cluster", st.Cluster)
	ctx = logr.NewContext(ctx, log)

	log.Info("start")

	st.update(v1.StateRunning, "upgrade k8s version")

	// get the current state of the node pools.
	pools, err := st.Azure.AKSNodepoolList(st.ResourceGroup, st.Cluster)
	if err != nil {
		st.error2(err, "az aks nodepool list")
		return
	}

	// make sure the pools are updated in a predictable (alphabetical) order.
	sort.Slice(pools, func(i, j int) bool { return pools[i].Name < pools[j].Name })

	var alreadyAtRightVersion int
	for _, pool := range pools {
		log := log.WithValues("pool", pool.Name)
		st.update(v1.StateRunning, "upgrade k8s version of pool "+pool.Name)

		switch pool.ProvisioningState {
		case azure.Succeeded:
			if pool.OrchestratorVersion == st.Version {
				alreadyAtRightVersion++
				continue
			}
		//case az.Failed: TODO retry?
		default: //az.Creating, az.Deleting, az.Migrating, az.Updating
			log.Error(fmt.Errorf("unexpected pool provision state: %s", pool.ProvisioningState), "bug")
			continue
		}

		// Disable autoscaling during upgrade.
		if pool.EnableAutoScaling {
			err = st.Azure.Autoscaler(false, pool.ResourceGroup, st.Cluster, pool.Name, pool.MinCount, pool.MaxCount)
			log.Error(err, "disable autoscaler on cluster %s pool %s", st.Cluster, pool.Name)
		}

		// Upgrade a pool.
		p, err := st.upgrade(ctx, pool.Name, log)
		if err != nil {
			st.error2(err, "upgrade k8s version")
			return
		}
		_ = p // we might want to show the pool after upgrade

		if pool.EnableAutoScaling {
			err = st.Azure.Autoscaler(true, pool.ResourceGroup, st.Cluster, pool.Name, pool.MinCount, pool.MaxCount)
			if err != nil {
				st.error2(err, "enable autoscaler")
				return
			}
		}
	}

	st.update(v1.StateReady, fmt.Sprintf("pools upgraded=%d, alreadyAtRightVersion=%d",
		len(pools)-alreadyAtRightVersion, alreadyAtRightVersion))
}

// Upgrade
func (st *AKSPoolStep) upgrade(_ context.Context, pool string, log logr.Logger) (*azure.AKSNodepool, error) {
	stop := make(chan bool)

	// start poller that provides status updates during the upgrade.
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				stop <- true
				return
			case <-ticker.C:
				p, err := st.Azure.AKSNodepool(st.ResourceGroup, st.Cluster, pool)
				if err != nil {
					log.Error(err, "poll nodepool")
					continue
				}
				log.Info("pool status", "cluster", st.Cluster, "pool", pool, "state", p.ProvisioningState)
			}
		}
	}()

	// start upgrade (slow)
	p, err := st.Azure.AKSNodepoolUpgrade(st.ResourceGroup, st.Cluster, pool, st.Version)

	// stop poller
	stop <- true
	<-stop

	return p, err
}
