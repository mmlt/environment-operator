package infra

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/terraform"
	"github.com/mmlt/environment-operator/pkg/tmplt"
)

// InitStep performs a terraform init
type InitStep struct {
	StepMeta

	/* Parameters */

	// Values to use for terraform input variables.
	Values InfraValues
	// SourcePath is the path to the directory containing terraform code.
	SourcePath string
	// Hash is an opaque value passed to Update.
	Hash string
}

// InfraValues hold the Specs that are needed during template expansion.
type InfraValues struct {
	Infra v1.InfraSpec
	Clusters []v1.ClusterSpec
}

// Meta returns a reference to the meta data this Step.
func (st *InitStep) Meta() *StepMeta {
	return &st.StepMeta
}

// Type returns the type of this Step.
func (st *InitStep) Type() string {
	return "InfraInit"
}

// ID returns a unique identification of this step.
func (st *InitStep) id() StepID {
	return st.ID
}

// Ord returns the execution order of this step.
func (st *InitStep) ord() StepOrd {
	return StepOrdInit
}

// Run a step.
func (st *InitStep) execute(ctx context.Context, isink Infoer, usink Updater, tf terraform.Terraformer, log logr.Logger) bool {
	log.Info("InitStep")

	// Run.
	st.State = StepStateRunning
	usink.Update(st)

	err := tmplt.ExpandAll(st.SourcePath, ".tmplt", st.Values)
	if err != nil {
		st.State = StepStateError
		st.Msg = err.Error()
		usink.Update(st)
		return false
	}

	tfr := tf.Init(st.SourcePath)

	// Return results.
	st.State = StepStateReady
	if tfr.Errors > 0 {
		st.State = StepStateError
	}

	st.Msg = fmt.Sprintf("terraform init errors=%d warnings=%d", tfr.Errors, tfr.Warnings)

	// TODO return values

	usink.Update(st)

	return st.State == StepStateReady
}
