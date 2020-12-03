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
	"github.com/mmlt/environment-operator/pkg/cloud"
	"github.com/mmlt/environment-operator/pkg/step"
	"k8s.io/apimachinery/pkg/types"
	"sync"
)

// Planner analyses the environment and decides what step should be executed next.
type Planner struct {
	sync.RWMutex

	// currentPlans keeps the most recent build Plan per environment.
	currentPlans map[types.NamespacedName]plan

	// AllowedStepTypes is a set of step types that are allowed to execute.
	// A nil set allows all steps.
	AllowedStepTypes map[step.Type]struct{}
	// Cloud provides generic cloud functions.
	Cloud cloud.Cloud
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
// Return false if no plan has been build for nsn yet.
func (p *Planner) currentPlan(nsn types.NamespacedName) (plan, bool) {
	p.RLock()
	defer p.RUnlock()

	if p.currentPlans == nil {
		return nil, false
	}

	pl, ok := p.currentPlans[nsn]

	return pl, ok
}

// CurrentPlanStep returns a Step by its short-name.
// Returns false if no such step is present (yet).
func (p *Planner) currentPlanStep(nsn types.NamespacedName, stepName string) (step.Step, bool) {
	pl, ok := p.currentPlan(nsn)
	if !ok {
		return nil, false
	}

	for _, st := range pl {
		if st.Meta().ID.ShortName() == stepName {
			return st, true
		}
	}

	return nil, false
}
