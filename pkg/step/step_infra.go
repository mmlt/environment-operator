package step

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-logr/logr"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/client/azure"
	"github.com/mmlt/environment-operator/pkg/client/terraform"
	"github.com/mmlt/environment-operator/pkg/cloud"
	"github.com/mmlt/environment-operator/pkg/tmplt"
	"github.com/mmlt/environment-operator/pkg/util"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// InfraStep performs a terraform init, plan and apply.
type InfraStep struct {
	Metaa

	/* Parameters */

	// Values to use for terraform input variables.
	Values InfraValues
	// SourcePath is the path to the directory containing terraform code.
	SourcePath string
	// Cloud provides generic cloud functionality.
	Cloud cloud.Cloud
	// Azure provides Azure resource manager functionality.
	// (prefer to use Cloud instead of Azure)
	Azure azure.AZer
	// Terraform provides terraform functionality.
	Terraform terraform.Terraformer

	/* Results */

	// Added, Changed, Deleted are then number of infrastructure objects affected.
	Added, Changed, Deleted int
}

// InfraValues hold the Specs that are needed during template expansion.
type InfraValues struct {
	Infra    v1.InfraSpec
	Clusters []v1.ClusterSpec
}

// Run a step.
func (st *InfraStep) Execute(ctx context.Context, env []string) {
	log := logr.FromContext(ctx).WithName("InfraStep")
	ctx = logr.NewContext(ctx, log)
	log.Info("start")

	st.update(v1.StateRunning, "terraform init")

	writeJSON(st.Values, st.SourcePath, "values.json", log)

	err := tmplt.ExpandAll(st.SourcePath, ".tmplt", st.Values)
	if err != nil {
		st.error2(err, "tmplt")
		return
	}

	sp, err := st.Cloud.Login()
	if err != nil {
		st.error2(err, "login")
		return
	}
	xenv := terraformEnviron(sp, st.Values.Infra.State.Access)
	writeEnv(xenv, st.SourcePath, "infra.env", log) // useful when invoking terraform manually.
	env = util.KVSliceMergeMap(env, xenv)

	tfr := st.Terraform.Init(ctx, env, st.SourcePath)
	writeText(tfr.Text, st.SourcePath, "init.txt", log)
	if len(tfr.Errors) > 0 {
		st.error2(nil, "terraform init "+tfr.Errors[0] /*first error only*/)
		return
	}

	// Plan
	st.update(v1.StateRunning, "terraform plan")

	tfr = st.Terraform.Plan(ctx, env, st.SourcePath)
	writeText(tfr.Text, st.SourcePath, "plan.txt", log)
	if len(tfr.Errors) > 0 {
		st.error2(nil, "terraform plan "+tfr.Errors[0] /*first error only*/)
		return
	}

	st.Added = tfr.PlanAdded
	st.Changed = tfr.PlanChanged
	st.Deleted = tfr.PlanDeleted
	if st.Added == 0 && st.Changed == 0 && st.Deleted == 0 {
		st.update(v1.StateReady, "terraform plan: nothing to do")
		return
	}

	// Check budget.
	var msgs []string
	b := st.Values.Infra.Budget
	if b.AddLimit != nil && tfr.PlanAdded > int(*b.AddLimit) {
		msgs = append(msgs, fmt.Sprintf("added %d exceeds addLimit %d", tfr.PlanAdded, *b.AddLimit))
	}
	if b.UpdateLimit != nil && tfr.PlanChanged > int(*b.UpdateLimit) {
		msgs = append(msgs, fmt.Sprintf("changed %d exceeds updateLimit %d", tfr.PlanChanged, *b.UpdateLimit))
	}
	if b.DeleteLimit != nil && tfr.PlanDeleted > int(*b.DeleteLimit) {
		msgs = append(msgs, fmt.Sprintf("deleted %d exceeds deleteLimit %d", tfr.PlanDeleted, *b.DeleteLimit))
	}
	if len(msgs) > 0 {
		st.error2(nil, "plan limits exceeded: "+strings.Join(msgs, ", "))
		return
	}

	// Prevent the autoscaling from fighting a node pool update or delete.
	pools, err := st.Terraform.GetPlanPools(ctx, env, st.SourcePath)
	for _, p := range pools {
		if p.Action&(terraform.ActionUpdate|terraform.ActionDelete) == 0 {
			continue
		}
		err = st.Azure.Autoscaler(false, p.ResourceGroup, p.Cluster, p.Pool, p.MinCount, p.MaxCount)
		if err != nil {
			st.error2(err, "disable autoscaler")
			return
		}
	}

	// Apply
	st.update(v1.StateRunning, fmt.Sprintf("terraform apply adds=%d changes=%d deletes=%d",
		tfr.PlanAdded, tfr.PlanChanged, tfr.PlanDeleted))

	cmd, ch, err := st.Terraform.StartApply(ctx, env, st.SourcePath)
	if err != nil {
		st.error2(err, "start terraform apply")
		return
	}

	// notify sink while waiting for command completion.
	var last *terraform.TFApplyResult
	for r := range ch {
		last = &r
	}

	// Re-enable autoscaling after change.
	for _, p := range pools {
		if p.Action&terraform.ActionUpdate == 0 {
			continue
		}
		err = st.Azure.Autoscaler(true, p.ResourceGroup, p.Cluster, p.Pool, p.MinCount, p.MaxCount)
		if err != nil {
			st.error2(err, "enable autoscaler")
			return
		}
	}

	if cmd != nil {
		// real cmd (fakes are nil).
		err := cmd.Wait()
		if err != nil {
			log.Error(err, "wait terraform apply")
		}
	}

	if last != nil {
		writeText(last.Text, st.SourcePath, "apply.txt", log)
	}

	// Return results.
	if last == nil {
		st.error2(nil, "did not receive response from terraform apply")
		return
	}

	if len(last.Errors) > 0 {
		st.error2(nil, strings.Join(last.Errors, ", "))
		return
	}

	st.Added = last.TotalAdded
	st.Changed = last.TotalChanged
	st.Deleted = last.TotalDestroyed

	st.update(v1.StateReady, fmt.Sprintf("terraform apply errors=0 added=%d changed=%d deleted=%d",
		last.TotalAdded, last.TotalChanged, last.TotalDestroyed))
}

// TerraformEnviron returns terraform specific environment variables.
func terraformEnviron(sp *cloud.ServicePrincipal, access string) map[string]string {
	r := make(map[string]string)
	r["ARM_CLIENT_ID"] = sp.ClientID
	r["ARM_CLIENT_SECRET"] = sp.ClientSecret
	r["ARM_TENANT_ID"] = sp.Tenant
	r["ARM_ACCESS_KEY"] = access
	return r
}

// WriteText writes text to dir/log/name.
// Errors are logged.
func writeText(text, dir, name string, log logr.Logger) {
	p := filepath.Join(dir, "log")
	err := os.MkdirAll(p, os.ModePerm)
	if err != nil {
		log.Info("writeText", "error", err)
		return
	}
	err = ioutil.WriteFile(filepath.Join(p, name), []byte(text), os.ModePerm)
	if err != nil {
		log.Info("writeText", "error", err)
	}
}

// WriteEnv writes env to dir/log/name.
// Errors are logged.
func writeEnv(env map[string]string, dir, name string, log logr.Logger) {
	s := "export"
	for k, v := range env {
		s = fmt.Sprintf("%s %s=%s", s, k, v)
	}
	writeText(s, dir, name, log)
}

// WriteJSON writes json to dir/log/name.
// Errors are logged.
func writeJSON(js interface{}, dir, name string, log logr.Logger) {
	b, err := json.MarshalIndent(js, "", "  ")
	if err != nil {
		log.Info("writeJSON", "error", err)
		return
	}
	writeText(string(b), dir, name, log)
}
