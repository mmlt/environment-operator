package terraform

import (
	"context"
	"fmt"
	"github.com/Jeffail/gabs/v2"
	"github.com/go-logr/logr"
	"github.com/mmlt/environment-operator/pkg/util/exe"
	"strings"
)

// GetPlanPools reads an existing plan and returns AKSPools that are going to be updated or deleted.
func (t *Terraform) GetPlanPools(ctx context.Context, env []string, dir string) ([]AKSPool, error) {
	log := logr.FromContext(ctx).WithName("GetPlanPools")

	o, _, err := exe.Run(log, &exe.Opt{Dir: dir, Env: env}, "", "terraform", "show",
		"-json", planName)
	if err != nil {
		return nil, err
	}

	return parseShowResponsePools(o)
}

// ParseShowResponsePools returns AKSPools that are going to be created, updated or deleted.
// The input string is json formatted conform https://www.terraform.io/docs/internals/json-format.html
func parseShowResponsePools(js string) ([]AKSPool, error) {
	obj, err := gabs.ParseJSON([]byte(js))
	if err != nil {
		return nil, err
	}

	var r []AKSPool
	for _, chg := range obj.Path("resource_changes").Children() {
		if chg.Path("type").Data().(string) != "azurerm_kubernetes_cluster_node_pool" {
			// not a pool.
			continue
		}

		act := stringsToAction(chg.Path("change.actions").Children())
		if act == 0 {
			// no change
			continue
		}

		chgBefore := chg.Path("change.before")
		if chgBefore.Data() == nil {
			// no change before
			continue
		}

		var minCount, maxCount int
		if v, ok := chgBefore.Path("min_count").Data().(float64); ok {
			minCount = int(v)
		}
		if v, ok := chgBefore.Path("max_count").Data().(float64); ok {
			maxCount = int(v)
		}

		if minCount == maxCount {
			// not an autoscaling pool
			continue
		}

		id := chgBefore.Path("id").Data().(string)
		m, err := pathToMap(id)
		if err != nil {
			return nil, err
		}

		r = append(r, AKSPool{
			ResourceGroup: m["resourcegroups"],
			Cluster:       m["managedClusters"],
			Pool:          m["agentPools"],
			MinCount:      minCount,
			MaxCount:      maxCount,
			Action:        act,
		})
	}

	return r, err
}

// StringsToAction maps a slice of action strings to an Action bitmap.
// https://www.terraform.io/docs/internals/json-format.html#change-representation
func stringsToAction(in []*gabs.Container) (act Action) {
	for _, a := range in {
		switch a.Data().(string) {
		case "create":
			act |= ActionCreate
		case "update":
			act |= ActionUpdate
		case "delete":
			act |= ActionDelete
		}
	}
	return
}

// PathToMap returns a map with key-value pairs from p.
// For example "/key1/value1/key2/value2" results in {"key1":"value1", "key2:"value2"}
func pathToMap(p string) (map[string]string, error) {
	ss := strings.Split(p, "/")
	ss = ss[1:] // a leading slash results an extra empty element

	if len(ss) == 0 {
		return nil, fmt.Errorf("empty input")
	}

	if len(ss)%2 != 0 {
		return nil, fmt.Errorf("expected even number of elements")
	}

	r := make(map[string]string)
	for i := 0; i < len(ss)-1; i += 2 {
		r[ss[i]] = ss[i+1]
	}
	return r, nil
}

// AKSPool represents an AKS Node pool change.
type AKSPool struct {
	ResourceGroup string
	Cluster       string
	Pool          string
	MinCount      int
	MaxCount      int
	Action        Action
}

// Action is the terraform plan action.
type Action int

const (
	ActionCreate Action = 1 << iota
	ActionUpdate
	ActionDelete
)
