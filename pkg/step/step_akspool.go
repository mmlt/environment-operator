package step

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/client/az"
	"github.com/mmlt/environment-operator/pkg/client/terraform"
	"time"
)

// AKSPoolStep can upgrade AKS node pools to the desired kubernetes version.
type AKSPoolStep struct {
	meta

	/* Parameters */

	// ResourceGroup that contains Cluster.
	ResourceGroup string
	// Cluster is the name of the AKS cluster to upgrade the node pool(s) of.
	// NB. We prefix AZ resources with the environment name.
	Cluster string
	// Version is the Kubernetes version to upgrade the node pool(s) to.
	Version string
}

// Meta returns a reference to the meta data this Step.
func (st *AKSPoolStep) Meta() *meta {
	return &st.meta
}

// Execute node pool upgrade for a cluster.
func (st *AKSPoolStep) Execute(ctx context.Context, isink Infoer, usink Updater, _ terraform.Terraformer /*TODO remove*/, log logr.Logger) bool {
	log = log.WithName("az").WithValues("cluster", st.Cluster)
	log.Info("start")

	st.State = v1.StateRunning
	usink.Update(st)

	// get the current state of the node pools.
	azcli := az.CLI{ResourceGroup: st.ResourceGroup, Log: log}
	pools, err := azcli.AKSNodepoolList(st.Cluster)
	if err != nil {
		log.Error(err, "az aks nodepool list")
		isink.Warning(st.ID, "az aks nodepool list: "+err.Error())
		st.State = v1.StateError
		st.Msg = "az aks nodepool list:" + err.Error()
		usink.Update(st)
		return false
	}

	//TODO make sure the pools are updated in a predictable order.

	for _, pool := range pools {
		log := log.WithValues("pool", pool.Name)

		switch pool.ProvisioningState {
		case az.Succeeded:
			if pool.OrchestratorVersion == st.Version {
				// already up-to-date
				continue
			}
		//case az.Failed: TODO retry?
		default: //az.Creating, az.Deleting, az.Migrating, az.Updating
			log.Error(fmt.Errorf("unexpected pool provision state: %s", pool.ProvisioningState), "bug")
			continue
		}

		// Upgrade a pool
		p, err := st.upgrade(ctx, pool.Name, isink, log)
		if err != nil {
			log.Error(err, "upgrade")
			isink.Warning(st.ID, "upgrade:"+err.Error())
			st.State = v1.StateError
			st.Msg = "upgrade:" + err.Error()
			usink.Update(st)
			return false
		}
		_ = p //TODO collect the results?
	}

	st.State = v1.StateReady
	//TODO st.Msg = fmt.Sprintf("kubectl-tmplt errors=0 added=%d changed=%d deleted=%d", tA, tC, tD)

	usink.Update(st)

	return st.State == v1.StateReady
}

func (st *AKSPoolStep) upgrade(ctx context.Context, pool string, isink Infoer, log logr.Logger) (*az.AKSNodepool, error) {
	stop := make(chan bool)

	// start status poller
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				stop <- true
				return
			case <-ticker.C:
				c := az.CLI{ResourceGroup: st.ResourceGroup, Log: log}
				p, err := c.AKSNodepool(st.Cluster, pool)
				if err != nil {
					log.Error(err, "poll nodepool")
					continue
				}
				isink.Info(st.ID, fmt.Sprintf("%s %s %s", st.Cluster, pool, p.ProvisioningState))
			}
		}
	}()

	// start upgrade
	c := az.CLI{ResourceGroup: st.ResourceGroup, Log: log}
	p, err := c.AKSNodepoolUpgrade(st.Cluster, pool, st.Version)

	// stop poller
	stop <- true
	<-stop

	return p, err
}
