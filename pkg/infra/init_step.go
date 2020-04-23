package infra

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/mmlt/environment-operator/pkg/terraform"
)

// InitStep performs a terraform init
type InitStep struct {
	StepMeta

	// Parameters

	// SourcePath is the path to the directory containing terraform code.
	SourcePath string
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
