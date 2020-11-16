package step

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/client/terraform"
	"strings"
)

// DestroyStep performs a terraform destroy.
type DestroyStep struct {
	Metaa

	/* Parameters */

	// SourcePath is the path to the directory containing terraform code.
	SourcePath string

	// Terraform is the terraform implementation to use.
	Terraform terraform.Terraformer

	/* Results */

	// The number of objects added, changed and deleted (destroyed) on terraform destroy completion.
	Added, Changed, Deleted int
}

// Meta returns a reference to the Metaa data of this Step.
func (st *DestroyStep) Meta() *Metaa {
	return &st.Metaa
}

// Execute terraform destroy.
func (st *DestroyStep) Execute(ctx context.Context, env []string, isink Infoer, usink Updater, log logr.Logger) bool {
	log.Info("start")

	// Run
	cmd, ch, err := st.Terraform.StartDestroy(ctx, env, st.SourcePath)
	if err != nil {
		log.Error(err, "start terraform destroy")
		isink.Warning(st.ID, "start terraform destroy:"+err.Error())
		st.State = v1.StateError
		st.Msg = "start terraform destroy:" + err.Error()
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
			log.Error(err, "wait terraform destroy")
		}
	}

	// Return results.
	if last == nil {
		st.State = v1.StateError
		st.Msg = "did not receive response from terraform destroy"
		usink.Update(st)
		return false
	}

	if len(last.Errors) > 0 {
		st.State = v1.StateError
		st.Msg = strings.Join(last.Errors, ", ")
	} else {
		st.State = v1.StateReady
		st.Msg = fmt.Sprintf("terraform destroy errors=0 added=%d changed=%d deleted=%d",
			last.TotalAdded, last.TotalChanged, last.TotalDestroyed)
	}

	st.Added = last.TotalAdded
	st.Changed = last.TotalChanged
	st.Deleted = last.TotalDestroyed

	// TODO return values (or check policies now and flag a warning)

	usink.Update(st)

	return st.State == v1.StateReady
}
