package infra

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/mmlt/environment-operator/pkg/terraform"
)

// PlanStep performs a terraform init
type PlanStep struct {
	StepMeta

	/* Parameters */

	// SourcePath is the path to the directory containing terraform code.
	SourcePath string
	// Hash is an opaque value passed to Update.
	Hash string

	/* Results */

	// Added, Changed, Deleted are then number of infrastructure objects affected when applying the plan.
	Added, Changed, Deleted int
}

// Meta returns a reference to the meta data this Step.
func (st *PlanStep) Meta() *StepMeta {
	return &st.StepMeta
}

// Type returns the type of this Step.
func (st *PlanStep) Type() string {
	return "InfraPlan"
}

/*// ID returns a unique identification of this step.
func (st *PlanStep) id() StepID {
	return st.ID
}*/

// Ord returns the execution order of this step.
func (st *PlanStep) ord() StepOrd {
	return StepOrdPlan
}

// Run a step.
func (st *PlanStep) execute(ctx context.Context, isink Infoer, usink Updater, tf terraform.Terraformer, log logr.Logger) bool {
	log.Info("PlanStep")

	// Run.
	st.State = StepStateRunning
	usink.Update(st)

	tfr := tf.Plan(st.SourcePath)

	// Return results.
	st.State = StepStateReady
	if tfr.Errors > 0 {
		st.State = StepStateError
	}

	st.Msg = fmt.Sprintf("terraform plan errors=%d warnings=%d added=%d changed=%d deleted=%d",
		tfr.Errors, tfr.Warnings, tfr.PlanAdded, tfr.PlanChanged, tfr.PlanDeleted)

	st.Added = tfr.PlanAdded
	st.Changed = tfr.PlanChanged
	st.Deleted = tfr.PlanDeleted

	usink.Update(st)

	return st.State == StepStateReady
}
