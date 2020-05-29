package step

/*TODO remove
func New(id ID, ispec v1.InfraSpec, cspec []v1.ClusterSpec, path, hash string) (Step, error) {
	var r Step

	switch id.Type {
	case TypeInit:
		r = &InitStep{
			Values: InfraValues{
				Infra:    ispec,
				Clusters: cspec,
			},
			SourcePath: path,
		}
	case TypePlan:
		r = &PlanStep{}
	case TypeApply:
		r = &ApplyStep{}
	//TODO case TypePool:
	//	r = &PoolStep{}
	case TypeKubeconfig:
		r = &KubeconfigStep{
		  	TFPath:      path,
		  	ClusterName: id.ClusterName,
		  	KCPath:      kcPath,
		}
	case TypeAddons:
		r = &AddonStep{
			SourcePath: path,
			KCPath:     kcPath,
			JobPaths:   cspec.Addons.Jobs,
			Values:     cspec.Addons.X,
			Hash:       hashAsString(hash),
			Addon:      addon,
		}
	default:
		return nil, fmt.Errorf("unexpected step: %v", id.Type)
	}

	r.Meta().ID = id
	r.Meta().Hash = hash

	return r, nil
}
*/
