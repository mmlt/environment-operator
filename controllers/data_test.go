package controllers

import (
	v1 "github.com/mmlt/environment-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// This file contains data used by multiple tests.

func testEnvironmentCR(nn types.NamespacedName, spec *v1.EnvironmentSpec) *v1.Environment {
	return &v1.Environment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nn.Name,
			Namespace: nn.Namespace,
		},
		Spec: *spec,
	}
}

// TestSpec1 for testing value overrides.
func testSpec1() *v1.EnvironmentSpec {
	return &v1.EnvironmentSpec{
		Infra: v1.InfraSpec{
			AZ: v1.AZSpec{
				Subscription: []v1.AZSubscription{
					{Name: "dummy", ID: "12345"},
				},
			},
			Source: v1.SourceSpec{
				Type: "local",
				URL:  "testdata/terraform", // relative to dir containing this _test.go file.
			},
			Main: "main.tf",
		},
		Defaults: v1.ClusterSpec{
			Infra: v1.ClusterInfraSpec{
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
			},
		},
		Clusters: []v1.ClusterSpec{
			{
				Name: "cpe",
				Infra: v1.ClusterInfraSpec{
					X: map[string]string{
						"overridden": "cpe-cluster",
					},
				},
			}, {
				Name: "second",
				Infra: v1.ClusterInfraSpec{
					X: map[string]string{
						"overridden": "second-cluster",
					},
				},
			},
		},
	}
}

// TestSpecLocal for all steps ICW with local k8s cluster.
func testSpecLocal() *v1.EnvironmentSpec {
	return &v1.EnvironmentSpec{

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
		Clusters: []v1.ClusterSpec{
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
					X: map[string]string{
						"k8sDomain": "xyz",
					},
				},
			},
		},
	}
}

// TestSpecLocalDestroy for Destroy ICW with local k8s cluster.
func testSpecLocalDestroy() *v1.EnvironmentSpec {
	cr := testSpecLocal()
	cr.Destroy = true
	x := int32(99)
	cr.Infra.Budget.DeleteLimit = &x
	return cr
}
