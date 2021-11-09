package controllers

import (
	v1 "github.com/mmlt/environment-operator/api/v1"
)

// This file contains data used by multiple tests.

//TestSpec1 for testing value overrides.
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
				URL:  "../e2e/testdata/terraform", // relative to dir containing this _test.go file.
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
					URL:  "../e2e/testdata/addons", // relative to dir containing this _test.go file.
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
