// Package Planner analyses the environment and decides what step should be executed next.
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

// Planner decides what next step needs to be taken to move the environment to desired state.
// It can be viewed as a mediator between the Environment custom resource (containing current and desired state) and
// the steps executor.
// Planner NextStep() maps Environment.Spec and .Status to a Step to be executed next.
// Planner Update() puts the result of a Step in the Environment.Status fields.
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
