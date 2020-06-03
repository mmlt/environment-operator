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

func (p *Planner) nextStep(nsn types.NamespacedName, src source.Getter, ispec v1.InfraSpec, cspec []v1.ClusterSpec, status v1.EnvironmentStatus) (step.Step, error) {
	plan, ok := p.currentPlan(nsn)
	if !ok {
		return nil, fmt.Errorf("expected plan for: %v", nsn)
	}

	for _, id := range plan {
		// Calculate hash over source and spec(s)
		// TODO consider moving calculation to a function that mirrors how newStep uses parameters.
		sh, err := src.Hash(nsn, id.ClusterName)
		if err != nil {
			return nil, err
		}
		var hash string
		if id.ClusterName == "" {
			// hash for infra steps
			//TODO move out of loop
			h := []interface{}{sh.Sum(nil), ispec}
			for _, s := range cspec {
				h = append(h, s.Infra)
			}
			hash, err = hashToString(h)
		} else {
			// hash for steps that have cluster specific parameters
			spec, err := cspecAtName(cspec, id.ClusterName)
			if err != nil {
				return nil, err
			}
			hash, err = hashToString(sh.Sum(nil), spec)
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
	// already performed the Get. TODO make Get call smarter to reduce copying.

	var r step.Step

	switch id.Type {
	case step.TypeInit:
		path, err := src.Get(nsn, "")
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
		path, err := src.Get(nsn, id.ClusterName)
		if err != nil {
			return nil, err
		}
		r = &step.PlanStep{
			SourcePath: path,
		}
	case step.TypeApply:
		path, err := src.Get(nsn, id.ClusterName)
		if err != nil {
			return nil, err
		}
		r = &step.ApplyStep{
			SourcePath: path,
		}
	case step.TypeAKSPool:
		spec, err := cspecAtName(cspec, id.ClusterName)
		if err != nil {
			return nil, err
		}
		r = &step.AKSPoolStep{
			ResourceGroup: ispec.AZ.ResourceGroup,
			Cluster:       ispec.EnvName + "-" + id.ClusterName, // we prefix AZ resources with env name.
			Version:       spec.Infra.Version,
		}
	case step.TypeKubeconfig:
		tfPath, err := src.Get(nsn, "")
		if err != nil {
			return nil, err
		}
		path, err := src.Get(nsn, id.ClusterName)
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
		path, err := src.Get(nsn, id.ClusterName)
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
