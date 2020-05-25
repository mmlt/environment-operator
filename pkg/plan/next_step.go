package plan

import (
	"encoding/base64"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/source"
	"github.com/mmlt/environment-operator/pkg/step"
	"hash"
	"k8s.io/apimachinery/pkg/types"
)

// NextStep takes kind Environment as an input and decides what Step should be executed next.
// Current state is stored as hashes of source code and parameters in the Environment kind status.
func (p *Plan) NextStep(nsn types.NamespacedName, src source.Getter, ispec v1.InfraSpec, cspec []v1.ClusterSpec, status v1.EnvironmentStatus) (step.Step, error) {
	st, err := p.nextStep(nsn, src, ispec, cspec, status)

	if st != nil {
		// set the fields common to all steps.
		id := &st.Meta().ID
		//id.Type = st.Type()
		id.Namespace = nsn.Namespace
		id.Name = nsn.Name
	}

	return st, err
}

func (p *Plan) nextStep(nsn types.NamespacedName, src source.Getter, ispec v1.InfraSpec, cspec []v1.ClusterSpec, status v1.EnvironmentStatus) (step.Step, error) {
	/*
		infraHash
		for step := init..apply poolupgrade (infra step iterator)
			if step.hasIssue
				if hasIssueBudget(step)
					step.Reset()
				else
					return nil, nil //nothing to do
			if step.Hash != infraHash
				return NewStep(stepname, steptype, spec, hash)

		for step := addon..qa (cluster step iterator)
			the same as above but with different input hash and spec

	*/

	//TODO remove path, h, err := src.Get(nsn, source.Ninfra)
	h, err := src.Hash(nsn, source.Ninfra)
	if err != nil {
		return nil, err
	}
	//TODO add parameters to hash
	hash := hashAsString(h)

	for _, id := range infraStepOrder(cspec) {
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

		if current.HasIssue() /*TODO introduce error budgets && !enoughIssueBudget(currentStatus)*/ {
			// no budget to retry
			return nil, nil
		}

		return p.stepNew(id, nsn, src, ispec, cspec, hash)
	}

	//TODO do the same for clusterStepOrder(cspec)

	return nil, nil
}

func (p *Plan) stepNew(id step.StepID, nsn types.NamespacedName, src source.Getter, ispec v1.InfraSpec, cspec []v1.ClusterSpec, hash string) (step.Step, error) {
	path, _, err := src.Get(nsn, source.Ninfra) //TODO make Ninfra a param?
	if err != nil {
		return nil, err
	}

	return step.New(id, ispec, cspec, path, hash)
}

// HashAsString returns the base64 representation of h.
func hashAsString(h hash.Hash) string {
	if h == nil {
		return ""
	}
	r := h.Sum(nil)
	return base64.StdEncoding.EncodeToString(r)
}
