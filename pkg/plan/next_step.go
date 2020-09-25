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

// NextStep decides what Step should be executed next.
// Current state is stored as hashes of source code and parameters in the Environment kind status.
// When a step hash doesn't match the hash stored in status the step will be executed.
func (p *Planner) NextStep(nsn types.NamespacedName, src source.Getter, destroy bool, ispec v1.InfraSpec, cspec []v1.ClusterSpec, status v1.EnvironmentStatus) (step.Step, error) {
	log := p.Log.WithName("NextStep").V(2)

	err := p.buildPlan(nsn, src, destroy, ispec, cspec)
	if err != nil {
		return nil, err
	}

	st, err := p.selectStep(nsn, status)

	if st != nil {
		log.Info("success", "stepName", st.Meta().ID.ShortName())
	}

	return st, err
}

// BuildPlan builds a plan containing the steps to create/update/delete a target environment.
// An environment is identified by nsn.
func (p *Planner) buildPlan(nsn types.NamespacedName, src source.Getter, destroy bool, ispec v1.InfraSpec, cspec []v1.ClusterSpec) error {
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
func (p *Planner) buildDestroyPlan(nsn types.NamespacedName, src source.Getter, ispec v1.InfraSpec, cspec []v1.ClusterSpec) error {
	pl := make(plan, 0, 3+4*len(cspec))

	tfPath, err := src.Get(nsn, "")
	if err != nil {
		return err
	}

	tfHash, err := src.Hash(nsn, "")
	if err != nil {
		return err
	}

	h := p.hash(tfHash)

	pl = append(pl,
		&step.InitStep{
			Metaa: stepMeta(nsn, "", step.TypeInit, h),
			Values: step.InfraValues{
				Infra:    ispec,
				Clusters: cspec,
			},
			SourcePath: tfPath,
			Terraform:  p.Terraform,
		},
		//TODO consider alternative of plan -destroy and apply
		&step.DestroyStep{
			Metaa:      stepMeta(nsn, "", step.TypeDestroy, h),
			SourcePath: tfPath,
			Terraform:  p.Terraform,
		})

	p.currentPlans[nsn] = pl

	return nil
}

// BuildCreatePlan builds a plan to create or update a target environment.
func (p *Planner) buildCreatePlan(nsn types.NamespacedName, src source.Getter, ispec v1.InfraSpec, cspec []v1.ClusterSpec) error {
	pl := make(plan, 0, 3+4*len(cspec))

	tfPath, err := src.Get(nsn, "")
	if err != nil {
		return err
	}

	// infra hash
	tfHash, err := src.Hash(nsn, "")
	if err != nil {
		return err
	}
	var cspecInfra []interface{}
	for _, s := range cspec {
		cspecInfra = append(cspecInfra, s.Infra)
	}
	h := p.hash(tfHash, ispec, cspecInfra)

	pl = append(pl,
		&step.InitStep{
			Metaa: stepMeta(nsn, "", step.TypeInit, h),
			Values: step.InfraValues{
				Infra:    ispec,
				Clusters: cspec,
			},
			SourcePath: tfPath,
			Terraform:  p.Terraform,
		},
		&step.PlanStep{
			Metaa:      stepMeta(nsn, "", step.TypePlan, h),
			SourcePath: tfPath,
			Terraform:  p.Terraform,
		},
		&step.ApplyStep{
			Metaa:      stepMeta(nsn, "", step.TypeApply, h),
			SourcePath: tfPath,
			Terraform:  p.Terraform,
		})

	for _, cl := range cspec {
		//TODO Fix that Hash and Git have to be called in order (Get depends on Hash having run once)
		cHash, err := src.Hash(nsn, cl.Name)
		if err != nil {
			return err
		}

		cPath, err := src.Get(nsn, cl.Name)
		if err != nil {
			return err
		}

		kcPath := filepath.Join(cPath, "kubeconfig")
		mvPath := filepath.Join(cPath, "mkv", "real") // TODO should be configurable in environment.yaml as it depends on k8s-cluster-addons repo layout

		pl = append(pl,
			&step.AKSPoolStep{
				Metaa:         stepMeta(nsn, cl.Name, step.TypeAKSPool, p.hash(tfHash, ispec.AZ.ResourceGroup, cl.Infra.Version)),
				ResourceGroup: ispec.AZ.ResourceGroup,
				Cluster:       prefixedClusterName("aks", ispec.EnvName, cl.Name),
				Version:       cl.Infra.Version,
				Azure:         p.Azure,
			},
			&step.KubeconfigStep{
				Metaa:       stepMeta(nsn, cl.Name, step.TypeKubeconfig, p.hash(tfHash)),
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
				Metaa:           stepMeta(nsn, cl.Name, step.TypeAddons, p.hash(cHash, cl.Addons.Jobs, cl.Addons.X)),
				SourcePath:      cPath,
				KCPath:          kcPath,
				MasterVaultPath: mvPath,
				JobPaths:        cl.Addons.Jobs,
				Values:          cl.Addons.X,
				Addon:           p.Addon,
			},
			//TODO step.TestStep
		)
	}

	p.currentPlans[nsn] = pl

	return nil
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

		/*// Return a Step even if we know it's running, it's up to the executor to accept the step or not.
		if current.State == v1.StateRunning {
			//TODO running for a long time may indicate a problem; for example the step execution stopped without updating the status
			return nil, nil
		}*/

		if current.State == v1.StateError {
			//TODO introduce error budgets && !enoughIssueBudget(currentStatus) to retry after error

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
