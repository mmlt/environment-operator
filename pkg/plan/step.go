package plan

import (
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/step"
)

func infraStepOrder(cspec []v1.ClusterSpec) []step.StepID {
	r := []step.StepID{
		{Type: step.StepTypeInit},
		{Type: step.StepTypePlan},
		{Type: step.StepTypeApply},
	}
	//TODO implement StepPool
	//for _, v := range cspec {
	//	r = append(r, step.StepID{Type: step.StepTypePool, ClusterName: v.Name})
	//}
	return r
}

func clusterStepOrder(cspec []v1.ClusterSpec) []step.StepID {
	r := make([]step.StepID, len(cspec)*3)
	for _, v := range cspec {
		r = append(r,
			step.StepID{Type: step.StepTypeKubeconfig, ClusterName: v.Name},
			step.StepID{Type: step.StepTypeAddons, ClusterName: v.Name},
			step.StepID{Type: step.StepTypeTest, ClusterName: v.Name})
	}
	return r
}

func allSteps(cspec []v1.ClusterSpec) []step.StepID {
	return append(infraStepOrder(cspec), clusterStepOrder(cspec)...)
}
