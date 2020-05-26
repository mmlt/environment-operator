package step

import (
	"context"
	"fmt"
	"github.com/ghodss/yaml"
	"github.com/go-logr/logr"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/addon"
	"github.com/mmlt/environment-operator/pkg/terraform"
	"io/ioutil"
	"path/filepath"
	"strings"
)

// AddonStep performs a kubectl-tmplt coterraform apply.
type AddonStep struct {
	meta

	Addon addon.Addonr

	/* Parameters */
	// KCPath is the path of the kube config file.
	KCPath string
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

// Meta returns a reference to the meta data this Step.
func (st *AddonStep) Meta() *meta {
	return &st.meta
}

// Execute addon.
func (st *AddonStep) Execute(ctx context.Context, isink Infoer, usink Updater, _ terraform.Terraformer /*TODO remove*/, log logr.Logger) bool {
	log.Info("ClusterAddon")

	st.State = v1.StateRunning
	usink.Update(st)

	// Create values yaml
	values, err := st.valuesYamlIn(st.SourcePath)
	if err != nil {
		log.Error(err, "addon")
		isink.Warning(st.ID, "addon:"+err.Error())
		st.State = v1.StateError
		st.Msg = "addon:" + err.Error()
		usink.Update(st)
		return false
	}

	var totals []addon.KTResult
	for _, job := range st.JobPaths {
		// Run kubectl-tmplt
		cmd, ch, err := st.Addon.Start(ctx, st.SourcePath, job, values, st.KCPath)
		if err != nil {
			log.Error(err, "start kubectl-tmplt")
			isink.Warning(st.ID, "start kubectl-tmplt:"+err.Error())
			st.State = v1.StateError
			st.Msg = "start kubectl-tmplt:" + err.Error()
			usink.Update(st)
			return false
		}

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

		//TODO fix 'last' not being set because no data is received from the channel since the Start().
		if last != nil {
			totals = append(totals, *last)
		}
	}

	// Return results.
	if len(totals) == 0 {
		st.State = v1.StateError
		st.Msg = "did not receive response from kubectl-tmplt"
		usink.Update(st)
		return false
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
		st.State = v1.StateError
		st.Msg = strings.Join(tE, ", ")
	} else {
		st.State = v1.StateReady
		st.Msg = fmt.Sprintf("kubectl-tmplt errors=0 added=%d changed=%d deleted=%d", tA, tC, tD)
	}

	st.Added = tA
	st.Changed = tC
	st.Deleted = tD

	usink.Update(st)

	return st.State == v1.StateReady
}

// ValuesYamlIn write a yaml file with st values and returns the path.
func (st *AddonStep) valuesYamlIn(dir string) (string, error) {
	const filename = "envreconvalues.yaml"

	d, err := yaml.Marshal(st.Values)
	if err != nil {
		return "", err
	}

	if string(d) == "null" {
		d = []byte{}
	}

	p := filepath.Join(dir, filename)
	err = ioutil.WriteFile(p, d, 0644)

	return p, err
}
