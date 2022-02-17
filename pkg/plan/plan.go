// Package planner plans the steps that needs to be taken to move an Environment to desired state.
package plan

import (
	"fmt"
	"github.com/go-logr/logr"
	"github.com/mitchellh/hashstructure"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/client/addon"
	"github.com/mmlt/environment-operator/pkg/client/azure"
	"github.com/mmlt/environment-operator/pkg/client/kubectl"
	"github.com/mmlt/environment-operator/pkg/client/terraform"
	"github.com/mmlt/environment-operator/pkg/cloud"
	"github.com/mmlt/environment-operator/pkg/cluster"
	"github.com/mmlt/environment-operator/pkg/source"
	"github.com/mmlt/environment-operator/pkg/step"
	"k8s.io/apimachinery/pkg/types"
	"path/filepath"
	"strconv"
)

// Planner plans the steps that are going to be executed.
type Planner struct {
	// currentPlans keeps the most recent build Plan per environment.
	currentPlans map[types.NamespacedName]plan

	// AllowedStepTypes is a set of step types that are allowed to execute.
	// A nil set allows all steps.
	AllowedStepTypes map[step.Type]struct{}
	// Cloud provides generic cloud access functions.
	Cloud cloud.Cloud
	// Terraform is the terraform implementation to use.
	Terraform terraform.Terraformer
	// Kubectl is the kubectl implementation to use.
	Kubectl kubectl.Kubectrler
	// Azure is the azure cli implementation to use.
	Azure azure.AZer
	// Addon can deploy k8s addon resources.
	Addon addon.Addonr
	// Client can access the cluster envop is running in.
	Client cluster.Client

	Log logr.Logger
}

// Plan is a sequence of steps to perform.
type plan []step.Step

// Sourcer keeps source code.
type Sourcer interface {
	Workspace(nsn types.NamespacedName, name string) (source.Workspace, bool)
}

// Plan returns an ordered collection of steps.
// The step hash field reflects the current source/parameters for that step.
func (p *Planner) Plan(nsn types.NamespacedName, src Sourcer, destroy bool, ispec v1.InfraSpec, cspec []v1.ClusterSpec) ([]step.Step, error) {
	pl, ok := p.buildPlan(nsn, src, destroy, ispec, cspec)
	if !ok {
		return nil, nil
	}

	if p.currentPlans == nil {
		p.currentPlans = make(map[types.NamespacedName]plan)
	}
	p.currentPlans[nsn] = pl

	return pl, nil
}

// BuildPlan builds a plan containing the steps to create/update/delete a target environment.
// An environment is identified by nsn.
// Returns false if not all prerequisites are fulfilled.
func (p *Planner) buildPlan(nsn types.NamespacedName, src Sourcer, destroy bool, ispec v1.InfraSpec, cspec []v1.ClusterSpec) (plan, bool) {
	var pl plan
	var ok bool
	switch {
	case destroy:
		pl, ok = p.buildDestroyPlan(nsn, src, ispec, cspec)

	default:
		pl, ok = p.buildCreatePlan(nsn, src, ispec, cspec, p.Client)
	}
	if !ok {
		return nil, false
	}

	return planFilter(pl, p.AllowedStepTypes), true
}

// BuildDestroyPlan builds a plan to delete a target environment.
// Returns false if workspaces are not prepped with sources.
func (p *Planner) buildDestroyPlan(nsn types.NamespacedName, src Sourcer, ispec v1.InfraSpec, cspec []v1.ClusterSpec) (plan, bool) {
	tfw, ok := src.Workspace(nsn, "")
	if !ok || tfw.Hash == "" {
		return nil, false
	}
	tfPath := filepath.Join(tfw.Path, ispec.Main)

	h := p.hash(tfw.Hash)

	pl := make(plan, 0, 1)
	pl = append(pl,
		&step.DestroyStep{
			Metaa: stepMeta(nsn, "", step.TypeDestroy, h),
			Values: step.InfraValues{
				Infra:    ispec,
				Clusters: cspec,
			},
			SourcePath: tfPath,
			Cloud:      p.Cloud,
			Terraform:  p.Terraform,
			Azure:      p.Azure,
		})

	return pl, true
}

// BuildCreatePlan builds a plan to create or update a target environment.
// Returns false if workspaces are not prepped with sources.
func (p *Planner) buildCreatePlan(nsn types.NamespacedName, src Sourcer, ispec v1.InfraSpec, cspec []v1.ClusterSpec, client cluster.Client) (plan, bool) {
	tfw, ok := src.Workspace(nsn, "")
	if !ok || !tfw.Synced {
		return nil, false
	}
	tfPath := filepath.Join(tfw.Path, ispec.Main)

	var cspecInfra []interface{}
	for _, s := range cspec {
		cspecInfra = append(cspecInfra, s.Infra)
	}
	h := p.hash(tfw.Hash, ispec, cspecInfra)

	pl := make(plan, 0, 1+4*len(cspec))
	pl = append(pl,
		&step.InfraStep{
			Metaa: stepMeta(nsn, "", step.TypeInfra, h),
			Values: step.InfraValues{
				Infra:    ispec,
				Clusters: cspec,
			},
			SourcePath: tfPath,
			Cloud:      p.Cloud,
			Azure:      p.Azure,
			Terraform:  p.Terraform,
			Client:     client,
			Kubectl:    p.Kubectl,
			KubeconfigPathFn: func(n string) (string, error) {
				cw, ok := src.Workspace(nsn, n)
				if !ok {
					return "", fmt.Errorf("no workspace for nsn=%v cluster=%v", nsn, n)
				}
				return filepath.Join(cw.Path, "kubeconfig"), nil
			},
		})

	for _, cl := range cspec {
		cw, ok := src.Workspace(nsn, cl.Name)
		if !ok || cw.Hash == "" {
			return nil, false
		}

		kcPath := filepath.Join(cw.Path, "kubeconfig")
		mvPath := filepath.Join(cw.Path, cl.Addons.MKV)

		az := p.Azure
		az.SetSubscription(ispec.AZ.Subscription[0].Name) // already validated
		pl = append(pl,
			&step.AKSPoolStep{
				Metaa:         stepMeta(nsn, cl.Name, step.TypeAKSPool, p.hash(tfw.Hash, ispec.AZ.ResourceGroup, cl.Infra.Version)),
				ResourceGroup: ispec.AZ.ResourceGroup,
				Cluster:       prefixedClusterName("aks", ispec.EnvName, cl.Name),
				Version:       cl.Infra.Version,
				Azure:         az,
			},
			&step.AKSAddonPreflightStep{
				Metaa:   stepMeta(nsn, cl.Name, step.TypeAKSAddonPreflight, h),
				KCPath:  kcPath,
				Kubectl: p.Kubectl,
			},
			&step.AddonStep{
				Metaa:           stepMeta(nsn, cl.Name, step.TypeAddons, p.hash(cw.Hash, cl.Addons.Jobs, cl.Addons.X)),
				SourcePath:      cw.Path,
				KCPath:          kcPath,
				MasterVaultPath: mvPath,
				JobPaths:        cl.Addons.Jobs,
				Values:          cl.Addons.X,
				Addon:           p.Addon,
			},
		)
	}

	return pl, true
}

// StepMeta is sugar for creating a step.Metaa struct.
func stepMeta(nsn types.NamespacedName, clusterName string, typ step.Type, hash string) step.Metaa {
	return step.Metaa{
		ID: step.ID{
			Type:        typ,
			Namespace:   nsn.Namespace,
			Name:        nsn.Name,
			ClusterName: clusterName,
		},
		Hash: hash,
	}
}

// Hash returns a string that is unique for args.
// Errors are logged but not returned.
func (p *Planner) hash(args ...interface{}) string {
	i, err := hashstructure.Hash(args, nil)
	if err != nil {
		p.Log.Error(err, "hash")
		return "hasherror"
	}
	return strconv.FormatUint(i, 16)
}

// PrefixedClusterName returns the name as it's used in Azure.
// NB. the same algo is in terraform
func prefixedClusterName(resource, env, name string) string {
	t := env[len(env)-1:]
	return fmt.Sprintf("%s%s001%s-%s", t, resource, env, name)
}

// PlanFilter returns plan with only the steps that are allowed.
// If allowed is nil plan is returned as-is.
func planFilter(pl plan, allowed map[step.Type]struct{}) plan {
	if len(allowed) == 0 {
		return pl
	}

	r := make(plan, 0, len(pl))
	for _, v := range pl {
		if _, ok := allowed[v.GetID().Type]; ok {
			r = append(r, v)
		}
	}

	return r
}

// CurrentPlan returns the Plan for a nsn.
// It returns false if no plan has been build for a nsn yet.
func (p *Planner) currentPlan(nsn types.NamespacedName) (plan, bool) {
	if p.currentPlans == nil {
		return nil, false
	}

	pl, ok := p.currentPlans[nsn]

	return pl, ok
}

// CurrentPlanStep returns a Step by its short-name.
// Returns false if no such step is present (yet).
func (p *Planner) currentPlanStep(nsn types.NamespacedName, stepName string) (step.Step, bool) {
	pl, ok := p.currentPlan(nsn)
	if !ok {
		return nil, false
	}

	for _, st := range pl {
		if st.GetID().ShortName() == stepName {
			return st, true
		}
	}

	return nil, false
}
