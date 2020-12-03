package step

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/client/terraform"
	"github.com/mmlt/environment-operator/pkg/cloud"
	"github.com/mmlt/environment-operator/pkg/tmplt"
	"github.com/mmlt/environment-operator/pkg/util"
	"strings"
)

// DestroyStep performs a terraform destroy.
type DestroyStep struct {
	Metaa

	/* Parameters */

	// Values to use for terraform input variables.
	Values InfraValues
	// SourcePath is the path to the directory containing terraform code.
	SourcePath string

	// Cloud provides generic cloud functionality.
	Cloud cloud.Cloud
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

// DeleteLimitForDestroy is the number the budget.deleteLimit must have for the Destroy to proceed.
// Any other number will deny Destroy.
const deleteLimitForDestroy = 99

// Execute terraform destroy.
func (st *DestroyStep) Execute(ctx context.Context, env []string, isink Infoer, usink Updater, log logr.Logger) bool {
	// Check budget.
	b := st.Values.Infra.Budget
	if b.DeleteLimit == nil || int(*b.DeleteLimit) != deleteLimitForDestroy {
		st.State = v1.StateError
		st.Msg = fmt.Sprintf("destroy requires budget.deleteLimit=%d to proceed", deleteLimitForDestroy)
		usink.Update(st)
		return false
	}

	log.Info("start")

	// Init
	// TODO refactor; similar code in step_destroy.go
	st.State = v1.StateRunning
	st.Msg = "terraform init"
	usink.Update(st)

	err := tmplt.ExpandAll(st.SourcePath, ".tmplt", st.Values)
	if err != nil {
		st.State = v1.StateError
		st.Msg = err.Error()
		usink.Update(st)
		return false
	}

	sp, err := st.Cloud.Login()
	if err != nil {
		st.State = v1.StateError
		st.Msg = err.Error()
		usink.Update(st)
		return false
	}
	xenv := terraformEnviron(sp, st.Values.Infra.State.Access)
	env = util.KVSliceMergeMap(env, xenv)

	tfr := st.Terraform.Init(ctx, env, st.SourcePath)
	writeText(tfr.Text, st.SourcePath, "init.txt", log)
	if len(tfr.Errors) > 0 {
		st.State = v1.StateError
		st.Msg = fmt.Sprintf("terraform init %s", tfr.Errors[0]) // first error only
		usink.Update(st)
		return false
	}

	// Destroy
	st.Msg = "terraform destroy"
	usink.Update(st)

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

	writeText(last.Text, st.SourcePath, "destroy.txt", log)

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
