package step

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/client/terraform"
	"github.com/mmlt/environment-operator/pkg/tmplt"
)

// InitStep performs a terraform init
type InitStep struct {
	Metaa

	/* Parameters */

	// Values to use for terraform input variables.
	Values InfraValues
	// SourcePath is the path to the directory containing terraform code.
	SourcePath string
	// Hash is an opaque value passed to Update.

	// Terraform is the terraform implementation to use.
	Terraform terraform.Terraformer
}

// InfraValues hold the Specs that are needed during template expansion.
type InfraValues struct {
	Infra    v1.InfraSpec
	Clusters []v1.ClusterSpec
}

// Meta returns a reference to the Metaa data this Step.
func (st *InitStep) Meta() *Metaa {
	return &st.Metaa
}

// Run a step.
func (st *InitStep) Execute(ctx context.Context, isink Infoer, usink Updater, log logr.Logger) bool {
	log.Info("start")

	// Run.
	st.State = v1.StateRunning
	usink.Update(st)

	err := tmplt.ExpandAll(st.SourcePath, ".tmplt", st.Values)
	if err != nil {
		st.State = v1.StateError
		st.Msg = err.Error()
		usink.Update(st)
		return false
	}

	tfr := st.Terraform.Init(st.SourcePath)

	// Return results.
	st.State = v1.StateReady
	if tfr.Errors > 0 {
		st.State = v1.StateError
	}

	st.Msg = fmt.Sprintf("terraform init errors=%d warnings=%d", tfr.Errors, tfr.Warnings)

	// TODO return values (or check policies now and flag a warning)

	usink.Update(st)

	return st.State == v1.StateReady
}
