package plan

import (
	"fmt"
	"github.com/mitchellh/hashstructure"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/source"
	"github.com/mmlt/environment-operator/pkg/step"
	"k8s.io/apimachinery/pkg/types"
	"path/filepath"
	"strconv"
)

type Sourcer interface {
	Workspace(nsn types.NamespacedName, name string) (source.Workspace, bool)
}

// NextStep decides what Step should be executed next.
// A nil Step is returned when there is no work to do (because prerequisites like sources are missing or the target is
// already up-to-date).
//
// Current state is stored as hashes of source code and parameters in the Environment kind status.
// When a step hash doesn't match the hash stored in status the step will be executed.
func (p *Planner) NextStep(nsn types.NamespacedName, src Sourcer, destroy bool, ispec v1.InfraSpec, cspec []v1.ClusterSpec, status v1.EnvironmentStatus) (step.Step, error) {
	ok := p.buildPlan(nsn, src, destroy, ispec, cspec)
	if !ok {
		return nil, nil
	}

	st, err := p.selectStep(nsn, status)

	if st != nil {
		p.Log.V(2).Info("NextStep", "request", nsn, "name", st.Meta().ID.ShortName())
	}

	return st, err
}

// BuildPlan builds a plan containing the steps to create/update/delete a target environment.
// An environment is identified by nsn.
// Returns false if not all prerequisites are fulfilled.
func (p *Planner) buildPlan(nsn types.NamespacedName, src Sourcer, destroy bool, ispec v1.InfraSpec, cspec []v1.ClusterSpec) bool {
	p.Lock()
	defer p.Unlock()

	if p.currentPlans == nil {
		p.currentPlans = make(map[types.NamespacedName]plan)
	}

	switch {
	case destroy:
		return p.buildDestroyPlan(nsn, src, ispec, cspec)

	default:
		return p.buildCreatePlan(nsn, src, ispec, cspec)
	}
}

// BuildDestroyPlan builds a plan to delete a target environment.
// Returns false if workspaces are not prepped with sources.
func (p *Planner) buildDestroyPlan(nsn types.NamespacedName, src Sourcer, ispec v1.InfraSpec, cspec []v1.ClusterSpec) bool {
	pl := make(plan, 0, 3+4*len(cspec))

	tfw, ok := src.Workspace(nsn, "")
	if !ok || tfw.Hash == "" {
		return false
	}
	tfPath := filepath.Join(tfw.Path, ispec.Main)

	h := p.hash(tfw.Hash)

	pl = append(pl,
		&step.DestroyStep{
			Metaa: stepMeta(nsn, "", step.TypeDestroy, h),
			Values: step.InfraValues{
				Infra:    ispec,
				Clusters: cspec,
			},
			SourcePath: tfPath,
			Terraform:  p.Terraform,
		})

	p.currentPlans[nsn] = pl

	return true
}

// BuildCreatePlan builds a plan to create or update a target environment.
func (p *Planner) buildCreatePlan(nsn types.NamespacedName, src Sourcer, ispec v1.InfraSpec, cspec []v1.ClusterSpec) bool {
	pl := make(plan, 0, 1+4*len(cspec))

	tfw, ok := src.Workspace(nsn, "")
	if !ok || tfw.Hash == "" {
		return false
	}
	tfPath := filepath.Join(tfw.Path, ispec.Main)

	var cspecInfra []interface{}
	for _, s := range cspec {
		cspecInfra = append(cspecInfra, s.Infra)
	}
	h := p.hash(tfw.Hash, ispec, cspecInfra)

	pl = append(pl,
		&step.InfraStep{
			Metaa: stepMeta(nsn, "", step.TypeInfra, h),
			Values: step.InfraValues{
				Infra:    ispec,
				Clusters: cspec,
			},
			SourcePath: tfPath,
			Terraform:  p.Terraform,
		})

	for _, cl := range cspec {
		cw, ok := src.Workspace(nsn, cl.Name)
		if !ok || cw.Hash == "" {
			return false
		}

		kcPath := filepath.Join(cw.Path, "kubeconfig")
		mvPath := filepath.Join(cw.Path, "mkv", "real") // TODO should be configurable in environment.yaml as it depends on k8s-cluster-addons repo layout

		az := p.Azure
		az.SetSubscription(ispec.AZ.Subscription)
		pl = append(pl,
			&step.AKSPoolStep{
				Metaa:         stepMeta(nsn, cl.Name, step.TypeAKSPool, p.hash(tfw.Hash, ispec.AZ.ResourceGroup, cl.Infra.Version)),
				ResourceGroup: ispec.AZ.ResourceGroup,
				Cluster:       prefixedClusterName("aks", ispec.EnvName, cl.Name),
				Version:       cl.Infra.Version,
				Azure:         az,
			},
			&step.KubeconfigStep{
				Metaa:       stepMeta(nsn, cl.Name, step.TypeKubeconfig, p.hash(tfw.Hash)),
				TFPath:      tfPath,
				ClusterName: cl.Name,
				KCPath:      kcPath,
				Terraform:   p.Terraform,
				Kubectl:     p.Kubectl,
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

	p.currentPlans[nsn] = pl

	return true
}

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

// SelectStep returns the next step to execute from current plan.
// NB. The returned Step might be in Running state (it's up to the executor to accept the step or not)
func (p *Planner) selectStep(nsn types.NamespacedName, status v1.EnvironmentStatus) (step.Step, error) {
	pl, ok := p.currentPlan(nsn)
	if !ok {
		return nil, fmt.Errorf("expected plan for: %v", nsn)
	}

	for _, st := range pl {
		id := st.Meta().ID

		// Get current step state.
		current, ok := status.Steps[id.ShortName()]
		if !ok {
			// first time this step is seen
			return st, nil
		}

		// Checking hash before state has the effect that steps that are in error state but not changed are skipped.
		// A step can get such a state when the following sequence of events take place;
		//	1. step source or parameter are changed
		//	2. step runs but errors
		//	3. changes from 1 are undone

		if current.Hash == st.Meta().Hash {
			continue
		}

		if current.State == v1.StateError {
			//TODO introduce error budgets to allow retry after error

			// no budget to retry
			return nil, nil
		}

		return st, nil
	}

	return nil, nil
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
