package step

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/terraform"
)

// PlanStep performs a terraform init
type PlanStep struct {
	meta

	/* Parameters */

	// SourcePath is the path to the directory containing terraform code.
	SourcePath string

	/* Results */

	// Added, Changed, Deleted are then number of infrastructure objects affected when applying the plan.
	Added, Changed, Deleted int
}

// Meta returns a reference to the meta data this Step.
func (st *PlanStep) Meta() *meta {
	return &st.meta
}

// Run a step.
func (st *PlanStep) Execute(ctx context.Context, isink Infoer, usink Updater, tf terraform.Terraformer, log logr.Logger) bool {
	log.Info("PlanStep")

	// Run.
	st.State = v1.StateRunning
	usink.Update(st)

	tfr := tf.Plan(st.SourcePath)

	// Return results.
	st.State = v1.StateReady
	if tfr.Errors > 0 {
		st.State = v1.StateError
	}

	st.Msg = fmt.Sprintf("terraform plan errors=%d warnings=%d added=%d changed=%d deleted=%d",
		tfr.Errors, tfr.Warnings, tfr.PlanAdded, tfr.PlanChanged, tfr.PlanDeleted)

	st.Added = tfr.PlanAdded
	st.Changed = tfr.PlanChanged
	st.Deleted = tfr.PlanDeleted

	// TODO return values (or check policies now and flag a warning)

	usink.Update(st)

	return st.State == v1.StateReady
}
