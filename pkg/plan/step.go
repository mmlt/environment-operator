package plan

import (
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/step"
)

func infraStepOrder(cspec []v1.ClusterSpec) []step.ID {
	r := []step.ID{
		{Type: step.TypeInit},
		{Type: step.TypePlan},
		{Type: step.TypeApply},
	}
	//TODO implement StepPool
	//for _, v := range cspec {
	//	r = append(r, step.ID{Type: step.TypePool, ClusterName: v.Name})
	//}
	return r
}

func clusterStepOrder(cspec []v1.ClusterSpec) []step.ID {
	r := make([]step.ID, len(cspec)*3)
	for _, v := range cspec {
		r = append(r,
			step.ID{Type: step.TypeKubeconfig, ClusterName: v.Name},
			step.ID{Type: step.TypeAddons, ClusterName: v.Name},
			step.ID{Type: step.TypeTest, ClusterName: v.Name})
	}
	return r
}

func allSteps(cspec []v1.ClusterSpec) []step.ID {
	return append(infraStepOrder(cspec), clusterStepOrder(cspec)...)
}
