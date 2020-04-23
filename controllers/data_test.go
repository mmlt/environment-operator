package controllers

import (
	v1 "github.com/mmlt/environment-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// This file contains test data.

func testEnvironmentCR(nn types.NamespacedName, spec *v1.EnvironmentSpec) *v1.Environment {
	return &v1.Environment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nn.Name,
			Namespace: nn.Namespace,
		},
		Spec: *spec,
	}
}

func testSpec1() *v1.EnvironmentSpec {
	return &v1.EnvironmentSpec{
		Defaults: v1.ClusterSpec{
			Infrastructure: v1.InfrastructureSpec{
				Source: v1.SourceSpec{
					Type: "local",
					URL:  "../config/samples/terraform", // relative to dir containing this _test.go file.
				},
				Main: "main.tf.tmplt",
				X: map[string]string{
					"overridden":    "default",
					"notOverridden": "default",
				},
			},
		},
		Clusters: []v1.ClusterSpec{
			{
				Name: "cpe",
				Infrastructure: v1.InfrastructureSpec{
					X: map[string]string{
						"overridden": "cpe-cluster",
					},
				},
			}, {
				Name: "second",
				Infrastructure: v1.InfrastructureSpec{
					X: map[string]string{
						"overridden": "second-cluster",
					},
				},
			},
		},
	}
}
