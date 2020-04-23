package infra

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/mmlt/environment-operator/pkg/terraform"
	"strings"
)

// ApplyStep performs a terraform apply.
type ApplyStep struct {
	StepMeta

	// Parameters

	// SourcePath is the path to the directory containing terraform code.
	SourcePath string
	// Hash is for pass-trough only.
	Hash string

	// Results

	// The number of objects added, changed and deleted (destroyed) on terraform apply completion.
	Added, Changed, Deleted int
}

// Meta returns a reference to the meta data this Step.
func (st *ApplyStep) Meta() *StepMeta {
	return &st.StepMeta
}

// Type returns the type of this Step.
func (st *ApplyStep) Type() string {
	return "InfraApply"
}

// ID returns a unique identification of this step.
func (st *ApplyStep) id() StepID {
	return st.ID
}

// Ord returns the execution order of this step.
func (st *ApplyStep) ord() StepOrd {
	return StepOrdApply
}

// Execute terraform apply.
func (st *ApplyStep) execute(ctx context.Context, isink Infoer, usink Updater, tf terraform.Terraformer, log logr.Logger) bool {
	log.Info("ApplyStep")

	// Run
	cmd, ch, err := tf.StartApply(ctx, st.SourcePath)
	if err != nil {
		log.Error(err, "start terraform apply")
		isink.Warning(st.ID, "start terraform apply:"+err.Error())
		st.State = StepStateError
		st.Msg = "start terraform apply:" + err.Error()
		usink.Update(st)
		return false
	}

	st.State = StepStateRunning
	usink.Update(st)

	// notify sink while waiting for command completion.
	var last *terraform.TFApplyResult
	for r := range ch {
		if r.Object != "" {
			isink.Info(st.ID, r.Object+" "+r.Action)
		}
		last = &r
	}

	if cmd != nil {
		// not a fake cmd.
		cmd.Wait()
	}

	// Return results.
	if last == nil {
		st.State = StepStateError
		st.Msg = "did not receive response from terraform apply"
		usink.Update(st)
		return false
	}

	if len(last.Errors) > 0 {
		st.State = StepStateError
		st.Msg = strings.Join(last.Errors, ", ")
	} else {
		st.State = StepStateReady
		st.Msg = fmt.Sprintf("terraform apply errors=0 added=%d changed=%d deleted=%d",
			last.TotalAdded, last.TotalChanged, last.TotalDestroyed)
	}

	st.Added = last.TotalAdded
	st.Changed = last.TotalChanged
	st.Deleted = last.TotalDestroyed

	usink.Update(st)

	return st.State == StepStateReady
}
