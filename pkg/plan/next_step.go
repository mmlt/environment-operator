package plan

import (
	"encoding/base64"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/infra"
	"github.com/mmlt/environment-operator/pkg/source"
	"hash"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// NextStep takes kind Environment as an input and decides what Step should be executed next.
// Current state is stored as hashes of source code and parameters in the Environment kind.
func (p *Plan) NextStep(nsn types.NamespacedName, src source.Getter, spec []v1.ClusterSpec, status v1.EnvironmentStatus) (infra.Step, error) {
	st, err := p.nextStep(nsn, src, spec, status)

	if st != nil {
		// set the fields common to all steps.
		id := &st.Meta().ID
		id.Type = st.Type()
		id.Namespace = nsn.Namespace
		id.Name = nsn.Name
	}

	return st, err
}

// See NextStep
func (p *Plan) nextStep(nsn types.NamespacedName, src source.Getter, spec []v1.ClusterSpec, status v1.EnvironmentStatus) (infra.Step, error) {
	h, err := src.Hash(nsn, source.Ninfra)
	if err != nil {
		return nil, err
	}

	if hashAsString(h) != status.Infra.Hash {
		return p.nsInfraChanged(nsn, src, spec, status)
	}

	// no changes
	return nil, nil
}

// InfraChanged predicate.
// Return steps to provisions infra structure.
// - hash is the desired state hash.
func (p *Plan) nsInfraChanged(nsn types.NamespacedName, src source.Getter, spec []v1.ClusterSpec, status v1.EnvironmentStatus) (infra.Step, error) {
	// wrap status.Conditions for read only access.
	cond := &conditions{inner: status.Conditions}

	if cs := cond.collect("Test", metav1.ConditionTrue, v1.ReasonRunning); len(cs) > 0 {
		// One or more test steps are running.
		// Request to cancel them so infra deployment can start earlier.
		// TODO return multiple steps to cancel them all together?
		return nil, nil
	}

	if cond.any("Infra", metav1.ConditionTrue, v1.ReasonRunning) {
		// A step is running.
		return nil, nil
	}

	path, hash, err := src.Get(nsn, source.Ninfra)
	if err != nil {
		return nil, err
	}

	// InitStep
	if cond.unknown("Infra") {
		// day1
		return &infra.InitStep{
			SourcePath: path,
		}, nil
	}
	if t, tsr := cond.matches("Infra", metav1.ConditionFalse, v1.ReasonReady); t == tsr &&
		t >= 3 &&
		cond.after("InfraApply", "InfraPlan", "InfraInit") {
		// All Infra steps have ConditionFalse/ReasonReady and
		// ApplyStep is newer than PlanStep, PlanStep is newer then InitStep.
		return &infra.InitStep{
			SourcePath: path,
		}, nil
	}

	// PlanStep
	if cond.any("InfraInit", metav1.ConditionFalse, v1.ReasonReady) &&
		cond.after("InfraInit", "InfraPlan") {
		// A new InitStep has completed successfully.
		return &infra.PlanStep{
			SourcePath: path,
		}, nil
	}

	// ApplyStep
	if cond.any("InfraPlan", metav1.ConditionFalse, v1.ReasonReady) &&
		cond.after("InfraPlan", "InfraApply") {
		// Plan step has completed successfully.
		// Continue with Apply step. TODO if dry-run == false
		return &infra.ApplyStep{
			SourcePath: path,
			Hash:       hashAsString(hash),
		}, nil
	}

	// Nothing to do under the current conditions.
	return nil, nil
}

/*// InfraUnchanged predicate.
// Check if Addons have changed.
// Return steps to deploy cluster addons and test the cluster.
func (p *Plan) nsInfraUnchanged(source source.Getter, spec []v1.ClusterSpec, status v1.EnvironmentStatus) (infra.Step, error) {

	//TODO implement

	return nil, nil
}*/

// HashAsString returns the base64 representation of h.
func hashAsString(h hash.Hash) string {
	if h == nil {
		return ""
	}
	r := h.Sum(nil)
	return base64.StdEncoding.EncodeToString(r)
}
