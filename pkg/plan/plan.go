// Package Planner analyses the Environment and decides what Step should be executed next.
package plan

import (
	"github.com/go-logr/logr"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/client/addon"
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

	currentPlans map[types.NamespacedName]Plan

	// Dependencies

	Addon addon.Addonr

	Log logr.Logger
}

// Plan is a sequence of steps to perform.
type Plan []step.ID

// CurrentPlan returns the Plan for nsn.
// Return false if no plan has been selected yet for nsn.
func (p *Planner) currentPlan(nsn types.NamespacedName) (Plan, bool) {
	p.RLock()
	defer p.RUnlock()

	if p.currentPlans == nil {
		return nil, false
	}

	pl, ok := p.currentPlans[nsn]

	return pl, ok
}

// SelectPlan decides what plan to use for environment nsn.
func (p *Planner) selectPlan(nsn types.NamespacedName, cspec []v1.ClusterSpec) {
	p.Lock()
	defer p.Unlock()

	if p.currentPlans == nil {
		p.currentPlans = make(map[types.NamespacedName]Plan)
	}

	// The current naive implementation recalculates the steps of a plan on each invocation.
	// Other implementations might switch a plan for a new one only when a certain step is reached.
	r := make(Plan, 3+4*len(cspec))
	r = append(r,
		step.ID{Type: step.TypeInit},
		step.ID{Type: step.TypePlan},
		step.ID{Type: step.TypeApply},
	)
	for _, v := range cspec {
		r = append(r,
			step.ID{Type: step.TypeAKSPool, ClusterName: v.Name},
			step.ID{Type: step.TypeKubeconfig, ClusterName: v.Name},
			step.ID{Type: step.TypeAddons, ClusterName: v.Name},
			//TODO step.ID{Type: step.TypeTest, ClusterName: v.Name},
		)
	}

	p.currentPlans[nsn] = r
}
