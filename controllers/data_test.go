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
			Source: v1.SourceSpec{
				Type: "local",
				URL:  "../config/samples/terraform", // relative to dir containing this _test.go file.
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
					URL:  "../config/samples/addons", // relative to dir containing this _test.go file.
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

// TestSpecLocal for test runs ICW with local k8s cluster.
func testSpecLocal() *v1.EnvironmentSpec {
	return &v1.EnvironmentSpec{

		Infra: v1.InfraSpec{
			EnvName:   "local",
			EnvDomain: "example.com",

			Source: v1.SourceSpec{
				Type: "local",
				URL:  "../config/samples/terraform", // relative to dir containing this _test.go file.
			},
			Main: "main.tf",

			AAD: v1.AADSpec{
				TenantID:        "na",
				ServerAppID:     "na",
				ServerAppSecret: "na",
				ClientAppID:     "na",
			},
			AZ: v1.AZSpec{
				Subscription:  "dummy",
				ResourceGroup: "dummy",
				VNetCIDR:      "10.20.30.0/24",
				SubnetNewbits: 5,
				X: map[string]string{
					"extra": "value",
				},
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
					URL:  "../config/samples/addons", // relative to dir containing this _test.go file.
				},
				Jobs: []string{
					"cluster/local/minikube/all.yaml",
				},
				X: map[string]string{
					"owner":          "xyz",
					"costcenter":     "default",
					"environment":    "local",
					"cpe/gitops":     "envop",
					"k8sEnvironment": "local",
					"k8sDomain":      "xyz.com",
				},
			},
		},
		Clusters: []v1.ClusterSpec{
			{
				Name: "one",

				Infra: v1.ClusterInfraSpec{
					SubnetNum: 1,
					Pools: map[string]v1.NodepoolSpec{
						"default": v1.NodepoolSpec{Scale: 2, VMSize: "Standard_DS2_v2"},
					},
					X: map[string]string{
						"overridden": "one-cluster",
					},
				},
				Addons: v1.ClusterAddonSpec{
					X: map[string]string{
						"k8sDomain": "one",
					},
				},
			},
		},
	}
}
