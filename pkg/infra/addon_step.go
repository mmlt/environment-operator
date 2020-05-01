package infra

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/mmlt/environment-operator/pkg/addon"
	"github.com/mmlt/environment-operator/pkg/terraform"
	"strings"
)

// AddonStep performs a terraform apply.
type AddonStep struct {
	StepMeta

	Target addon.Addonr

	/* Parameters */

	// SourcePath is the path to the directory containing the k8s resources.
	SourcePath string
	// JobPath is the path (relative to SourcePath) to the yaml file containing steps.
	JobPath                 string
	// ValuesPath is the path (relative to SourcePath) to the yaml fie containing template values.
	ValuesPath              string
	// Hash is an opaque value passed to Update.
	Hash string

	/* Results */

	// The number of resources created, modified and deleted.
	Added, Changed, Deleted int

}

// Meta returns a reference to the meta data this Step.
func (st *AddonStep) Meta() *StepMeta {
	return &st.StepMeta
}

// Type returns the type of this Step.
func (st *AddonStep) Type() string {
	return "ClusterAddon"
}

// ID returns a unique identification of this step.
func (st *AddonStep) id() StepID {
	return st.ID
}

// Ord returns the execution order of this step.
func (st *AddonStep) ord() StepOrd {
	return StepOrdAddons
}

// Execute addon.
func (st *AddonStep) execute(ctx context.Context, isink Infoer, usink Updater, _ terraform.Terraformer, log logr.Logger) bool {
	log.Info("ClusterAddon")

	// TODO Get kubeconfig
	// tf.Output() ?
	kubeconfig := "path/to/kube/config"

	// Run kubectl-tmplt
	cmd, ch, err := st.Target.Start(ctx, st.SourcePath, st.JobPath, st.ValuesPath, kubeconfig)
	if err != nil {
		log.Error(err, "start kubectl-tmplt")
		isink.Warning(st.ID, "start kubectl-tmplt:"+err.Error())
		st.State = StepStateError
		st.Msg = "start kubectl-tmplt:" + err.Error()
		usink.Update(st)
		return false
	}

	st.State = StepStateRunning
	usink.Update(st)

	// notify sink while waiting for command completion.
	var last *addon.KTResult
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
			log.Error(err, "wait kubectl-tmplt")
		}
	}

	// Return results.
	if last == nil {
		st.State = StepStateError
		st.Msg = "did not receive response from kubectl-tmplt"
		usink.Update(st)
		return false
	}

	if len(last.Errors) > 0 {
		st.State = StepStateError
		st.Msg = strings.Join(last.Errors, ", ")
	} else {
		st.State = StepStateReady
		st.Msg = fmt.Sprintf("kubectl-tmplt errors=0 added=%d changed=%d deleted=%d",
			last.Added, last.Changed, last.Deleted)
	}

	st.Added = last.Added
	st.Changed = last.Changed
	st.Deleted = last.Deleted

	usink.Update(st)

	return st.State == StepStateReady
}
