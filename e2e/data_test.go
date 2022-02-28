package e2e

import (
	v1 "github.com/mmlt/environment-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"strconv"
)

// This file contains data used by multiple tests.

func testEnvironmentCR(nn types.NamespacedName, labelSet labels.Set, spec *v1.EnvironmentSpec) *v1.Environment {
	return &v1.Environment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nn.Name,
			Namespace: nn.Namespace,
			Labels:    labelSet,
		},
		Spec: *spec,
	}
}

// TestSpecLocal returns a spec to run all steps ICW with local k8s cluster.
// ClusterCnt defines how many clusters are in the spec.
func testSpecLocal(clusterCnt int) *v1.EnvironmentSpec {
	spec := &v1.EnvironmentSpec{
		Infra: v1.InfraSpec{
			EnvName:   "local",
			EnvDomain: "example.com",

			Source: v1.SourceSpec{
				Type: "local",
				URL:  "testdata/terraform", // relative to dir containing this _test.go file.
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
		},
		Defaults: v1.ClusterSpec{
			Infra: v1.ClusterInfraSpec{
				Version: "1.16.8",
				X: map[string]string{
					"overridden":    "default",
					"notOverridden": "default",
				},
			},
			Addons: v1.ClusterAddonSpec{
				Source: v1.SourceSpec{
					Type: "local",
					URL:  "testdata/addons", // relative to dir containing this _test.go file.
				},
				Jobs: []string{
					"cluster/local/minikube/all.yaml",
				},
				MKV: "mkv/fake",
				X: map[string]string{
					"owner":          "harry",
					"costcenter":     "default",
					"environment":    "local",
					"cpe/gitops":     "envop",
					"k8sEnvironment": "local",
					"k8sDomain":      "example.com",
				},
			},
		},
	}

	name := func(i int) string {
		if i > 1 {
			return "xyz" + strconv.Itoa(i)
		}
		return "xyz"
	}

	for i := 0; i < clusterCnt; i++ {
		spec.Clusters = append(spec.Clusters,
			v1.ClusterSpec{
				Name: name(i),

				Infra: v1.ClusterInfraSpec{
					SubnetNum: 1,
					Pools: map[string]v1.NodepoolSpec{
						"default": {Scale: 2, VMSize: "Standard_DS2_v2"},
					},
					X: map[string]string{
						"overridden": name(i) + "-cluster",
					},
				},
				Addons: v1.ClusterAddonSpec{
					X: map[string]string{
						"k8sCluster": name(i),
					},
				},
			})
	}

	return spec
}

// TestSpecLocalDestroy for Destroy ICW with local k8s cluster.
func testSpecLocalDestroy() *v1.EnvironmentSpec {
	cr := testSpecLocal(1)
	cr.Destroy = true
	x := int32(99)
	cr.Infra.Budget.DeleteLimit = &x
	return cr
}
