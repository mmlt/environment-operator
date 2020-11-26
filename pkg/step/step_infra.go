package step

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/client/terraform"
	"github.com/mmlt/environment-operator/pkg/tmplt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// InfraStep performs a terraform init, plan, apply
type InfraStep struct {
	Metaa

	/* Parameters */

	// Values to use for terraform input variables.
	Values InfraValues
	// SourcePath is the path to the directory containing terraform code.
	SourcePath string
	// Hash is an opaque value passed to Update.

	// Terraform is the terraform implementation to use.
	Terraform terraform.Terraformer

	/* Results */

	// Added, Changed, Deleted are then number of infrastructure objects affected when applying the plan.
	Added, Changed, Deleted int
}

// InfraValues hold the Specs that are needed during template expansion.
type InfraValues struct {
	Infra    v1.InfraSpec
	Clusters []v1.ClusterSpec
}

// Meta returns a reference to the Metaa data of this Step.
func (st *InfraStep) Meta() *Metaa {
	return &st.Metaa
}

// Run a step.
func (st *InfraStep) Execute(ctx context.Context, env []string, isink Infoer, usink Updater, log logr.Logger) bool {
	log.Info("start")

	// TODO review isink usage

	// Init
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

	tfr := st.Terraform.Init(ctx, env, st.SourcePath)
	writeText(st.SourcePath, "init.txt", tfr.Text, log)
	if len(tfr.Errors) > 0 {
		st.State = v1.StateError
		st.Msg = fmt.Sprintf("terraform init %s", tfr.Errors[0]) // first error only
		usink.Update(st)
		return false
	}

	// Plan
	st.Msg = "terraform plan"
	usink.Update(st)

	tfr = st.Terraform.Plan(ctx, env, st.SourcePath)
	writeText(st.SourcePath, "plan.txt", tfr.Text, log)
	if len(tfr.Errors) > 0 {
		st.State = v1.StateError
		st.Msg = fmt.Sprintf("terraform plan %s", tfr.Errors[0]) // first error only
		usink.Update(st)
		return false
	}

	st.Added = tfr.PlanAdded
	st.Changed = tfr.PlanChanged
	st.Deleted = tfr.PlanDeleted

	//TODO if all values are zero there is no point in running apply.

	// Check budget.
	b := st.Values.Infra.Budget
	if b.AddLimit != nil && tfr.PlanAdded > int(*b.AddLimit) {
		st.State = v1.StateError
		st.Msg = fmt.Sprintf("plan added %d exceeds addLimit %d", tfr.PlanAdded, *b.AddLimit)
		usink.Update(st)
		return false
	}
	if b.UpdateLimit != nil && tfr.PlanChanged > int(*b.UpdateLimit) {
		st.State = v1.StateError
		st.Msg = fmt.Sprintf("plan changed %d exceeds updateLimit %d", tfr.PlanChanged, *b.UpdateLimit)
		usink.Update(st)
		return false
	}
	if b.DeleteLimit != nil && tfr.PlanDeleted > int(*b.DeleteLimit) {
		st.State = v1.StateError
		st.Msg = fmt.Sprintf("plan deleted %d exceeds deleteLimit %d", tfr.PlanDeleted, *b.DeleteLimit)
		usink.Update(st)
		return false
	}

	// Apply
	st.Msg = fmt.Sprintf("terraform apply adds=%d changes=%d deletes=%d", tfr.PlanAdded, tfr.PlanChanged, tfr.PlanDeleted)
	usink.Update(st)

	cmd, ch, err := st.Terraform.StartApply(ctx, env, st.SourcePath)
	if err != nil {
		log.Error(err, "start terraform apply")
		isink.Warning(st.ID, "start terraform apply:"+err.Error())
		st.State = v1.StateError
		st.Msg = "start terraform apply:" + err.Error()
		usink.Update(st)
		return false
	}

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

	writeText(st.SourcePath, "apply.txt", last.Text, log)

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

	// TODO these values should not have changed
	st.Added = last.TotalAdded
	st.Changed = last.TotalChanged
	st.Deleted = last.TotalDestroyed

	usink.Update(st)

	return st.State == v1.StateReady
}

// WriteText writes text to dir/log/name.
// Errors are logged.
func writeText(dir, name, text string, log logr.Logger) {
	p := filepath.Join(dir, "log")
	err := os.MkdirAll(p, os.ModePerm)
	if err != nil {
		log.Info("InitStep", "error", err)
	}
	err = ioutil.WriteFile(filepath.Join(p, name), []byte(text), os.ModePerm)
	if err != nil {
		log.Info("InitStep", "error", err)
	}
}
