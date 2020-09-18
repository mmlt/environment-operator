// Package planner decides what step needs to be taken next to move the environment to desired state.
// Planner NextStep() uses the current and desired state of the environment as stored in the Environment custom resource
// to determine the next step. Planner UpdateStatus*() methods write Step progress back to Environment.Status fields.
// As such the planner is the one who reads/writes Environment custom resource fields.
package plan

import (
	"github.com/go-logr/logr"
	"github.com/mmlt/environment-operator/pkg/client/addon"
	"github.com/mmlt/environment-operator/pkg/client/azure"
	"github.com/mmlt/environment-operator/pkg/client/kubectl"
	"github.com/mmlt/environment-operator/pkg/client/terraform"
	"github.com/mmlt/environment-operator/pkg/step"
	"k8s.io/apimachinery/pkg/types"
	"sync"
)

// Planner analyses the environment and decides what step should be executed next.
type Planner struct {
	sync.RWMutex

	// currentPlans keeps a Plan for each environment managed so UpdateStatus can aggregate stats.
	currentPlans map[types.NamespacedName]plan //TODO why do we keep this? for conditions?

	// Terraform is the terraform implementation to use.
	Terraform terraform.Terraformer
	// Kubectl is the kubectl implementation to use.
	Kubectl kubectl.Kubectrler
	// Azure is the azure cli implementation to use.
	Azure azure.AZer
	// Addon can deploy k8s addon resources.
	Addon addon.Addonr

	Log logr.Logger
}

// Plan is a sequence of steps to perform.
type plan []step.Step

// CurrentPlan returns the Plan for nsn.
// Return false if no plan has been selected yet for nsn.
func (p *Planner) currentPlan(nsn types.NamespacedName) (plan, bool) {
	p.RLock()
	defer p.RUnlock()

	if p.currentPlans == nil {
		return nil, false
	}

	pl, ok := p.currentPlans[nsn]

	return pl, ok
}
