package plan

import (
	"fmt"
	"github.com/mitchellh/hashstructure"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/source"
	"github.com/mmlt/environment-operator/pkg/step"
	"k8s.io/apimachinery/pkg/types"
	"path/filepath"
	"strconv"
)

// NextStep takes kind Environment as an input and decides what Step should be executed next.
// Current state is stored as hashes of source code and parameters in the Environment kind status.
func (p *Planner) NextStep(nsn types.NamespacedName, src source.Getter, ispec v1.InfraSpec, cspec []v1.ClusterSpec, status v1.EnvironmentStatus) (step.Step, error) {
	p.selectPlan(nsn, cspec)

	st, err := p.nextStep(nsn, src, ispec, cspec, status)

	if st != nil {
		// set the fields common to all steps.
		id := &st.Meta().ID
		id.Namespace = nsn.Namespace
		id.Name = nsn.Name
	}

	return st, err
}

/*TODO remove
func (p *Planner) nextStepOLD(nsn types.NamespacedName, src source.Getter, ispec v1.InfraSpec, cspec []v1.ClusterSpec, status v1.EnvironmentStatus) (step.Step, error) {


		//infraHash
		//for step := init..apply poolupgrade (infra step iterator)
		//	if step.hasIssue
		//		if hasIssueBudget(step)
		//			step.Reset()
		//		else
		//			return nil, nil //nothing to do
		//	if step.Hash != infraHash
		//		return NewStep(stepname, steptype, spec, hash)
		//
		//for step := addon..qa (cluster step iterator)
		//	the same as above but with different input hash and spec

	h, err := src.Hash(nsn, "")
	if err != nil {
		return nil, err
	}
	//TODO add parameters to hash
	hash := hashAsString(h)

	plan, ok := p.currentPlan(nsn)
	if !ok {
		return nil, fmt.Errorf("expected plan for: %v", nsn)
	}

	for _, id := range plan {
		current, ok := status.Steps[id.ShortName()]
		if !ok {
			// first time this step is seen
			return p.stepNew(id, nsn, src, ispec, cspec, hash)
		}

		if current.Hash == hash {
			continue
		}

		if current.State == v1.StateRunning {
			continue
			//TODO running for a long time may indicate a problem; for example the step execution stopped without updating the status
		}

		if current.HasIssue() {
//TODO introduce error budgets && !enoughIssueBudget(currentStatus)
// no budget to retry
			return nil, nil
		}

		return p.stepNew(id, nsn, src, ispec, cspec, hash)
	}

	//TODO do the same for clusterStepOrder(cspec)

	return nil, nil
}
*/

func (p *Planner) nextStep(nsn types.NamespacedName, src source.Getter, ispec v1.InfraSpec, cspec []v1.ClusterSpec, status v1.EnvironmentStatus) (step.Step, error) {
	plan, ok := p.currentPlan(nsn)
	if !ok {
		return nil, fmt.Errorf("expected plan for: %v", nsn)
	}

	for _, id := range plan {
		h, err := src.Hash(nsn, id.ClusterName)
		if err != nil {
			return nil, err
		}
		// sum source and spec hash
		var hash string
		if id.ClusterName == "" {
			hash, err = hashToString(h.Sum(nil), ispec)
		} else {
			spec, err := cspecAtName(cspec, id.ClusterName)
			if err != nil {
				return nil, err
			}
			hash, err = hashToString(h.Sum(nil), spec)
		}
		if err != nil {
			return nil, err
		}

		// get current step state
		current, ok := status.Steps[id.ShortName()]
		if !ok {
			// first time this step is seen
			return p.stepNew(id, nsn, src, ispec, cspec, hash)
		}

		if current.State == v1.StateRunning {
			//TODO running for a long time may indicate a problem; for example the step execution stopped without updating the status
			return nil, nil
		} else if current.State == v1.StateError {
			/*TODO introduce error budgets && !enoughIssueBudget(currentStatus)*/

			// no budget to retry
			return nil, nil
		}

		if current.Hash == hash {
			continue
		}

		return p.stepNew(id, nsn, src, ispec, cspec, hash)
	}

	return nil, nil
}

func (p *Planner) stepNew(id step.ID, nsn types.NamespacedName, src source.Getter, ispec v1.InfraSpec, cspec []v1.ClusterSpec, hash string) (step.Step, error) {
	// NB. Source Get is called for each step that needs a source. Many of these calls are redundant as a previous call
	// already performed the Get. TODO The Get call can be made smarter to reduce copying.

	var r step.Step

	switch id.Type {
	case step.TypeInit:
		path, _, err := src.Get(nsn, "")
		if err != nil {
			return nil, err
		}
		r = &step.InitStep{
			Values: step.InfraValues{
				Infra:    ispec,
				Clusters: cspec,
			},
			SourcePath: path,
		}
	case step.TypePlan:
		path, _, err := src.Get(nsn, id.ClusterName)
		if err != nil {
			return nil, err
		}
		r = &step.PlanStep{
			SourcePath: path,
		}
	case step.TypeApply:
		path, _, err := src.Get(nsn, id.ClusterName)
		if err != nil {
			return nil, err
		}
		r = &step.ApplyStep{
			SourcePath: path,
		}
	//TODO case step.TypePool:
	//	r = &step.PoolStep{}
	case step.TypeKubeconfig:
		tfPath, _, err := src.Get(nsn, "")
		if err != nil {
			return nil, err
		}
		path, _, err := src.Get(nsn, id.ClusterName)
		if err != nil {
			return nil, err
		}
		kcPath := filepath.Join(path, "kubeconfig")
		r = &step.KubeconfigStep{
			TFPath:      tfPath,
			ClusterName: id.ClusterName,
			KCPath:      kcPath,
		}
	case step.TypeAddons:
		path, _, err := src.Get(nsn, id.ClusterName)
		if err != nil {
			return nil, err
		}
		kcPath := filepath.Join(path, "kubeconfig")
		spec, err := cspecAtName(cspec, id.ClusterName)
		if err != nil {
			return nil, err
		}
		r = &step.AddonStep{
			SourcePath: path,
			KCPath:     kcPath,
			JobPaths:   spec.Addons.Jobs,
			Values:     spec.Addons.X,
			Addon:      p.Addon,
		}
	default:
		return nil, fmt.Errorf("unexpected step type: %v", id.Type)
	}

	r.Meta().ID = id
	r.Meta().Hash = hash

	return r, nil
}

//TODO remove HashAsString returns the base64 representation of h.
/*func hashAsString(h hash.Hash) string {
	if h == nil {
		return ""
	}
	r := h.Sum(nil)
	return base64.StdEncoding.EncodeToString(r)
}*/

// HashToString returns a string that is unique for args.
func hashToString(args ...interface{}) (string, error) {
	i, err := hashstructure.Hash(args, nil)
	if err != nil {
		return "", err
	}
	return strconv.FormatUint(i, 16), nil
}

// CSpectAtName returns the ClusterSpec with name.
func cspecAtName(cspec []v1.ClusterSpec, name string) (*v1.ClusterSpec, error) {
	for _, s := range cspec {
		if s.Name == name {
			return &s, nil
		}
	}
	return nil, fmt.Errorf("no clusterSpec with name: %s", name)
}
