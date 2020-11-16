package step

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/client/terraform"
)

// PlanStep performs a terraform init
type PlanStep struct {
	Metaa

	/* Parameters */

	// SourcePath is the path to the directory containing terraform code.
	SourcePath string

	// Terraform is the terraform implementation to use.
	Terraform terraform.Terraformer

	/* Results */

	// Added, Changed, Deleted are then number of infrastructure objects affected when applying the plan.
	Added, Changed, Deleted int
}

// Meta returns a reference to the Metaa data of this Step.
func (st *PlanStep) Meta() *Metaa {
	return &st.Metaa
}

// Run a step.
func (st *PlanStep) Execute(ctx context.Context, env []string, isink Infoer, usink Updater, log logr.Logger) bool {
	log.Info("start")

	// Run.
	st.State = v1.StateRunning
	usink.Update(st)

	tfr := st.Terraform.Plan(ctx, env, st.SourcePath)

	writeText(st.SourcePath, "plan.txt", tfr.Text, log)

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
