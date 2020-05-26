package step

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/terraform"
	"strings"
)

// ApplyStep performs a terraform apply.
type ApplyStep struct {
	meta

	/* Parameters */

	// SourcePath is the path to the directory containing terraform code.
	SourcePath string

	/* Results */

	// The number of objects added, changed and deleted (destroyed) on terraform apply completion.
	Added, Changed, Deleted int
}

// Meta returns a reference to the meta data this Step.
func (st *ApplyStep) Meta() *meta {
	return &st.meta
}

// Execute terraform apply.
func (st *ApplyStep) Execute(ctx context.Context, isink Infoer, usink Updater, tf terraform.Terraformer, log logr.Logger) bool {
	log.Info("ApplyStep")

	// Run
	cmd, ch, err := tf.StartApply(ctx, st.SourcePath)
	if err != nil {
		log.Error(err, "start terraform apply")
		isink.Warning(st.ID, "start terraform apply:"+err.Error())
		st.State = v1.StateError
		st.Msg = "start terraform apply:" + err.Error()
		usink.Update(st)
		return false
	}

	st.State = v1.StateRunning
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
		// real cmd (fakes are nil).
		err := cmd.Wait()
		if err != nil {
			log.Error(err, "wait terraform apply")
		}
	}

	// Return results.
	if last == nil {
		st.State = v1.StateError
		st.Msg = "did not receive response from terraform apply"
		usink.Update(st)
		return false
	}

	if len(last.Errors) > 0 {
		st.State = v1.StateError
		st.Msg = strings.Join(last.Errors, ", ")
	} else {
		st.State = v1.StateReady
		st.Msg = fmt.Sprintf("terraform apply errors=0 added=%d changed=%d deleted=%d",
			last.TotalAdded, last.TotalChanged, last.TotalDestroyed)
	}

	st.Added = last.TotalAdded
	st.Changed = last.TotalChanged
	st.Deleted = last.TotalDestroyed

	// TODO return values (or check policies now and flag a warning)

	usink.Update(st)

	return st.State == v1.StateReady
}
