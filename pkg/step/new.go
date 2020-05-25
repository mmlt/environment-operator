package step

import (
	"fmt"
	v1 "github.com/mmlt/environment-operator/api/v1"
)

func New(id StepID, ispec v1.InfraSpec, cspec []v1.ClusterSpec, path, hash string) (Step, error) {
	var r Step

	switch id.Type {
	case StepTypeInit:
		r = &InitStep{
			Values: InfraValues{
				Infra:    ispec,
				Clusters: cspec,
			},
			SourcePath: path,
		}
	case StepTypePlan:
		r = &PlanStep{}
	case StepTypeApply:
		r = &ApplyStep{}

		/*TODO implements KubeconfigStep, AddonStep
		  infra.KubeconfigStep{
		  	TFPath:      tfPath,
		  	ClusterName: clusterName,
		  	KCPath:      kcPath,
		  }
		  		return &infra.AddonStep{
		  			SourcePath: path,
		  			KCPath:     kcPath,
		  			JobPaths:   cspec.Addons.Jobs,
		  			Values:     cspec.Addons.X,
		  			Hash:       hashAsString(hash),
		  			Addon:      addon,
		  		}*/
	default:
		return nil, fmt.Errorf("unexpected step: %v", id.Type)
	}

	r.Meta().ID = id
	r.Meta().Hash = hash

	return r, nil
}
