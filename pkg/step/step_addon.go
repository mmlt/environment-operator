package step

import (
	"context"
	"fmt"
	"github.com/ghodss/yaml"
	"github.com/go-logr/logr"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/client/addon"
	"io/ioutil"
	"path/filepath"
	"strings"
)

// AddonStep performs a kubectl-tmplt apply.
type AddonStep struct {
	Metaa

	Addon addon.Addonr

	/* Parameters */

	// KCPath is the path of the kube config file.
	KCPath string
	// MasterVaultPath is the path to a directory containing the config of the Vault to use.
	MasterVaultPath string
	// SourcePath is the path to the directory containing the k8s resources.
	SourcePath string
	// JobPaths is collection of paths (relative to SourcePath) to job files.
	// kubectl-tmplt is run for each element in the collection.
	JobPaths []string
	// Values are passed with -set flag to kubectl-tmplt.
	Values map[string]string

	/* Results */

	// The number of resources created, modified and deleted.
	Added, Changed, Deleted int
}

// Execute addon apply for a cluster.
func (st *AddonStep) Execute(ctx context.Context, env []string, log logr.Logger) {
	log.Info("start")

	st.update(v1.StateRunning, "values yaml")

	// Create values yaml
	values, err := st.valuesYamlIn(st.SourcePath)
	if err != nil {
		st.error2(err, "values yaml")
		return
	}

	var totals []addon.KTResult
	for _, job := range st.JobPaths {
		st.update(v1.StateRunning, job)

		// Start kubectl-tmplt
		cmd, ch, err := st.Addon.Start(ctx, env, st.SourcePath, job, values, st.KCPath, st.MasterVaultPath)
		if err != nil {
			st.error2(err, "start kubectl-tmplt")
			return
		}

		// notify sink while waiting for command completion.
		var last *addon.KTResult
		for r := range ch {
			last = &r
		}

		if cmd != nil {
			// real cmd (fakes are nil).
			err := cmd.Wait()
			if err != nil {
				log.Error(err, "wait kubectl-tmplt")
			}
		}

		if last == nil {
			// no data has been received from the channel since the Start().
			log.Info("kubectl-tmplt no feedback received")

			continue //TODO or exit loop?
		}

		totals = append(totals, *last)

		if len(last.Errors) > 0 {
			break
		}
	}

	// Return results.
	if len(totals) == 0 {
		st.error2(nil, "nothing applied")
		return
	}
	// aggregate totals
	var tE []string
	var tA, tC, tD int
	for _, t := range totals {
		tE = append(tE, t.Errors...)
		tA = +t.Added
		tC = +t.Changed
		tD = +t.Deleted
	}
	if len(tE) > 0 {
		st.error2(nil, strings.Join(tE, ", "))
		return
	}

	st.Added = tA
	st.Changed = tC
	st.Deleted = tD

	st.update(v1.StateReady, fmt.Sprintf("kubectl-tmplt errors=0 added=%d changed=%d deleted=%d", tA, tC, tD))
}

// ValuesYamlIn write a yaml file with st values and returns the path.
func (st *AddonStep) valuesYamlIn(dir string) (string, error) {
	const filename = "envopvalues.yaml"

	d := []byte{}
	if st.Values != nil {
		var err error
		d, err = yaml.Marshal(st.Values)
		if err != nil {
			return "", err
		}
	}

	p := filepath.Join(dir, filename)
	err := ioutil.WriteFile(p, d, 0644)

	return p, err
}
