package step

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/client/azure"
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
	// Azure is the azure cli implementation to use.
	Azure azure.AZer

	/* Results */

	// The number of objects added, changed and deleted (destroyed) on terraform destroy completion.
	Added, Changed, Deleted int
}

// DeleteLimitForDestroy is the number the budget.deleteLimit must have for the Destroy to proceed.
// Any other number will deny Destroy.
const deleteLimitForDestroy = 99

// Execute terraform destroy.
func (st *DestroyStep) Execute(ctx context.Context, env []string) {
	// Check budget.
	b := st.Values.Infra.Budget
	if b.DeleteLimit == nil || int(*b.DeleteLimit) != deleteLimitForDestroy {
		msg := fmt.Sprintf("destroy requires budget.deleteLimit=%d to proceed", deleteLimitForDestroy)
		st.error2(nil, msg)
		return
	}

	log := logr.FromContext(ctx).WithName("DestroyStep")
	ctx = logr.NewContext(ctx, log)
	log.Info("start")

	// Init
	st.update(v1.StateRunning, "terraform init")

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

	// Disable autoscaler(s)
	err = st.Azure.AllAutoscalers(false, st.Values.Clusters, st.Values.Infra.AZ.ResourceGroup, log)
	if err != nil {
		st.error2(err, "az aks nodepool list")
		return
	}

	// Destroy
	st.update(v1.StateRunning, "terraform destroy")

	cmd, ch, err := st.Terraform.StartDestroy(ctx, env, st.SourcePath)
	if err != nil {
		st.error2(err, "start terraform destroy")
		return
	}

	// keep last line of stdout/err
	var last *terraform.TFApplyResult
	for r := range ch {
		last = &r
	}

	if cmd != nil {
		// real cmd (fakes are nil).
		err := cmd.Wait()
		if err != nil {
			log.Error(err, "wait terraform destroy")
		}
	}

	if last != nil {
		writeText(last.Text, st.SourcePath, "destroy.txt", log)
	}

	// Return results.
	if last == nil {
		st.error2(nil, "did not receive response from terraform destroy")
		return
	}

	if len(last.Errors) > 0 {
		st.error2(nil, strings.Join(last.Errors, ", "))
		return
	}

	st.Added = last.TotalAdded
	st.Changed = last.TotalChanged
	st.Deleted = last.TotalDestroyed

	st.update(v1.StateReady, fmt.Sprintf("terraform destroy errors=0 added=%d changed=%d deleted=%d",
		last.TotalAdded, last.TotalChanged, last.TotalDestroyed))
}
