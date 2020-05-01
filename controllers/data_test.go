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
