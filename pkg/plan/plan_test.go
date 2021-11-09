package plan

import (
	"github.com/go-logr/stdr"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/client/azure"
	"github.com/mmlt/environment-operator/pkg/source"
	"github.com/mmlt/environment-operator/pkg/step"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/types"
	"log"
	"os"
	"reflect"
	"testing"
)

func Test_planFilter(t *testing.T) {
	nsn := metav1.NamespacedName{
		Namespace: "default",
		Name:      "test",
	}

	planInfraDestroy := plan{
		&step.InfraStep{
			Metaa: stepMeta(nsn, "", step.TypeInfra, ""),
		},
		&step.DestroyStep{
			Metaa: stepMeta(nsn, "", step.TypeDestroy, ""),
		},
	}

	type args struct {
		pl      plan
		allowed map[step.Type]struct{}
	}
	tests := []struct {
		it   string
		args args
		want plan
	}{
		{
			it: "should not remove steps when allowed is nil",
			args: args{
				pl:      planInfraDestroy,
				allowed: nil,
			},
			want: planInfraDestroy,
		},
		{
			it: "should not remove steps when allowed is empty",
			args: args{
				pl:      planInfraDestroy,
				allowed: map[step.Type]struct{}{},
			},
			want: planInfraDestroy,
		},
		{
			it: "should have allowed steps only",
			args: args{
				pl: planInfraDestroy,
				allowed: map[step.Type]struct{}{
					step.TypeDestroy: {},
				},
			},
			want: plan{
				&step.DestroyStep{
					Metaa: stepMeta(nsn, "", step.TypeDestroy, ""),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.it, func(t *testing.T) {
			got := planFilter(tt.args.pl, tt.args.allowed)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestPlanner_Plan_ignore_parameters asserts the planner returns the correct steps based on step.Metaa.
// It doesn't not assert the step specific parameters are correctly set.
func TestPlanner_Plan_ignore_parameters(t *testing.T) {
	nsn := metav1.NamespacedName{
		Namespace: "default",
		Name:      "test",
	}

	type args struct {
		nsn              metav1.NamespacedName
		allowedStepTypes map[step.Type]struct{}
		src              Sourcer
		destroy          bool
		ispec            v1.InfraSpec
		cspec            []v1.ClusterSpec
	}
	tests := []struct {
		it   string
		args args
		want []step.Metaa
	}{
		{
			it: "should return steps for infra creation",
			args: args{
				nsn: nsn,
				src: fakeSource{
					workspace: source.Workspace{
						Path:   "does/not/matter",
						Hash:   "9999",
						Synced: true,
					},
				},
				destroy: false,
				ispec:   infraSpec("does/not/matter"),
				cspec:   nil,
			},
			want: []step.Metaa{
				{
					ID: step.ID{
						Type:        step.TypeInfra,
						Namespace:   "default",
						Name:        "test",
						ClusterName: "",
					},
					Hash: "b134dc8eca86e844", //"b2e166111bf1c016",
				},
			},
		},
		{
			it: "should return steps for infra creation with a hash that changes with source hash",
			args: args{
				nsn: nsn,
				src: fakeSource{
					workspace: source.Workspace{
						Path:   "does/not/matter",
						Hash:   "9990",
						Synced: true,
					},
				},
				destroy: false,
				ispec:   infraSpec("does/not/matter"),
				cspec:   nil,
			},
			want: []step.Metaa{
				{
					ID: step.ID{
						Type:        step.TypeInfra,
						Namespace:   "default",
						Name:        "test",
						ClusterName: "",
					},
					Hash: "1762976ecb35a230", //"4b76faac2401a19f",
				},
			},
		},
		{
			it: "should return step for infra destroy",
			args: args{
				nsn: nsn,
				src: fakeSource{
					workspace: source.Workspace{
						Path:   "does/not/matter",
						Hash:   "9990",
						Synced: true,
					},
				},
				destroy: true,
				ispec:   infraSpec("does/not/matter"),
				cspec:   nil,
			},
			want: []step.Metaa{
				{
					ID: step.ID{
						Type:        step.TypeDestroy,
						Namespace:   "default",
						Name:        "test",
						ClusterName: "",
					},
					Hash: "68b850e1b0c7cf04",
				},
			},
		},
		{
			it: "should return steps for infra and cluster create",
			args: args{
				nsn: nsn,
				src: fakeSource{
					workspace: source.Workspace{
						Path:   "does/not/matter",
						Hash:   "9990",
						Synced: true,
					},
				},
				destroy: false,
				ispec:   infraSpec("does/not/matter"),
				cspec:   clusterSpec("does/not/matter/either"),
			},
			want: []step.Metaa{
				{
					ID: step.ID{
						Type:        step.TypeInfra,
						Namespace:   "default",
						Name:        "test",
						ClusterName: "",
					},
					Hash: "ff311b4a7990bdc2", //"f122548f6c981695",
				},
				{
					ID: step.ID{
						Type:        step.TypeAKSPool,
						Namespace:   "default",
						Name:        "test",
						ClusterName: "xyz",
					},
					Hash: "bade927d58b84e23",
				},
				{
					ID: step.ID{
						Type:        step.TypeKubeconfig,
						Namespace:   "default",
						Name:        "test",
						ClusterName: "xyz",
					},
					Hash: "68b850e1b0c7cf04",
				},
				{
					ID: step.ID{
						Type:        step.TypeAKSAddonPreflight,
						Namespace:   "default",
						Name:        "test",
						ClusterName: "xyz",
					},
					Hash: "ff311b4a7990bdc2", //"f122548f6c981695",
				},
				{
					ID: step.ID{
						Type:        step.TypeAddons,
						Namespace:   "default",
						Name:        "test",
						ClusterName: "xyz",
					},
					Hash: "ae8dab454f8aa2a",
				},
			},
		},
	}

	l := stdr.New(log.New(os.Stdout, "", log.Lshortfile|log.Ltime))

	for _, tt := range tests {
		t.Run(tt.it, func(t *testing.T) {
			p := &Planner{
				AllowedStepTypes: tt.args.allowedStepTypes,
				Azure:            &azure.AZFake{},

				Log: l,
			}
			got, err := p.Plan(tt.args.nsn, tt.args.src, tt.args.destroy, tt.args.ispec, tt.args.cspec)

			// collect metaa struct refs
			var gotmeta []*step.Metaa
			for _, st := range got {
				v := reflect.ValueOf(st).Elem()
				for i := 0; i < v.NumField(); i++ {
					n := v.Type().Field(i).Name
					if n == "Metaa" {
						m := v.Field(i).Addr().Interface().(*step.Metaa)
						gotmeta = append(gotmeta, m)
						break
					}
				}
			}

			if assert.NoError(t, err) {
				var want [](*step.Metaa)
				for i, _ := range tt.want {
					want = append(want, &tt.want[i])
				}
				assert.Equal(t, want, gotmeta)
			}
		})
	}
}

// TestPlanner_Plan_step_hash check that changes to infra and cluster specs result in hash changes.
func TestPlanner_Plan_step_hash(t *testing.T) {
	tests := []struct {
		id          string
		mutateISpec func(*v1.InfraSpec)
		mutateCSpec func(*[]v1.ClusterSpec)
		want        []string
	}{
		{
			id: "ispec change triggers Infra step",
			mutateISpec: func(ispec *v1.InfraSpec) {
				ispec.EnvDomain = "xxx"
			},
			want: []string{"Infra", "AKSAddonPreflightxyz"},
		},
		{
			id: "add pool to cluster triggers Infra step",
			mutateCSpec: func(cspec *[]v1.ClusterSpec) {
				(*cspec)[0].Infra.Pools["new"] = (*cspec)[0].Infra.Pools["default"]
			},
			want: []string{"Infra", "AKSAddonPreflightxyz"},
		},
		{
			id: "add 2nd cluster triggers Infra step",
			mutateCSpec: func(cspec *[]v1.ClusterSpec) {
				cl := clusterSpec("dont/care")[0]
				cl.Name = "new"
				*cspec = append(*cspec, cl)
			},
			want: []string{"Infra", "AKSAddonPreflightxyz", "AKSPoolnew", "Kubeconfignew", "AKSAddonPreflightnew", "Addonsnew"},
		},
		{
			id: "cspec Addons change trigger Addons step",
			mutateCSpec: func(cspec *[]v1.ClusterSpec) {
				(*cspec)[0].Addons.X["key"] = "values"
			},
			want: []string{"Addonsxyz"},
		},
	}

	l := stdr.New(log.New(os.Stdout, "", log.Lshortfile|log.Ltime))

	nsn := metav1.NamespacedName{
		Namespace: "default",
		Name:      "test",
	}

	src := fakeSource{
		workspace: source.Workspace{
			Path:   "does/not/matter",
			Hash:   "9999",
			Synced: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			p := &Planner{
				Azure: &azure.AZFake{},
				Log:   l,
			}

			ispec := infraSpec("does/not/matter")
			cspec := clusterSpec("does/not/matter/either")

			p1, err := p.Plan(nsn, src, false, ispec, cspec)
			assert.NoError(t, err)

			if tt.mutateISpec != nil {
				tt.mutateISpec(&ispec)
			}
			if tt.mutateCSpec != nil {
				tt.mutateCSpec(&cspec)
			}

			p2, err := p.Plan(nsn, src, false, ispec, cspec)
			assert.NoError(t, err)

			// compare the step hashes of both plans and collect the names of the steps that have changed.
			pm1 := planAsMap(p1)
			var changed []string
			for _, s2 := range p2 {
				n := s2.GetID().ShortName()
				s1, ok := pm1[n]
				if !ok {
					changed = append(changed, n)
					continue
				}

				if s1.GetHash() != s2.GetHash() {
					changed = append(changed, n)
				}
			}

			assert.Equal(t, tt.want, changed)
		})
	}
}

// InfraSpec returns a InfraSpec with Source set to src.
// If src is a relative path id's relative to the dir containing this _test.go file.
func infraSpec(src string) v1.InfraSpec {
	return v1.InfraSpec{
		EnvName:   "local",
		EnvDomain: "example.com",

		Source: v1.SourceSpec{
			Type: "local",
			URL:  src,
		},
		Main: "main.tf",

		AAD: v1.AADSpec{
			TenantID:        "na",
			ServerAppID:     "na",
			ServerAppSecret: "na",
			ClientAppID:     "na",
		},
		AZ: v1.AZSpec{
			Subscription: []v1.AZSubscription{
				{Name: "dummy", ID: "12345"},
			},
			ResourceGroup: "dummy",
			VNetCIDR:      "10.20.30.0/24",
			SubnetNewbits: 5,
		},
	}
}

// ClusterSpec returns a slice of ClusterSpecs with Source set to src.
// If src is a relative path id's relative to the dir containing this _test.go file.
func clusterSpec(src string) []v1.ClusterSpec {
	return []v1.ClusterSpec{
		{
			Name: "xyz",

			Infra: v1.ClusterInfraSpec{
				SubnetNum: 1,
				Pools: map[string]v1.NodepoolSpec{
					"default": {Scale: 2, VMSize: "Standard_DS2_v2"},
				},
				X: map[string]string{
					"overridden": "xyz-cluster",
				},
			},
			Addons: v1.ClusterAddonSpec{
				Source: v1.SourceSpec{
					Type: "local",
					URL:  src,
				},
				X: map[string]string{
					"k8sDomain": "xyz",
				},
			},
		},
	}
}

// FakeSource is a Sourcer for testing that just returns a workspace.
type fakeSource struct {
	workspace source.Workspace
}

func (f fakeSource) Workspace(_ metav1.NamespacedName, _ string) (source.Workspace, bool) {
	return f.workspace, true
}

var _ Sourcer = fakeSource{}

// planAsMap returns a map of steps indexed by their ShortName.
func planAsMap(p []step.Step) map[string]step.Step {
	r := make(map[string]step.Step, len(p))
	for _, s := range p {
		r[s.GetID().ShortName()] = s
	}
	return r
}
