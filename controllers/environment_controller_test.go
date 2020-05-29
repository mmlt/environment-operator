package controllers

import (
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestController_flattenedClusterSpec(t *testing.T) {
	tests := []struct {
		it   string
		in   v1.EnvironmentSpec
		want []v1.ClusterSpec
	}{
		{
			it: "should_override_default_values",
			in: *testSpec1(),
			want: []v1.ClusterSpec{
				{
					Name: "cpe",
					Infra: v1.ClusterInfraSpec{
						X: map[string]string{
							"notOverridden": "default",
							"overridden":    "cpe-cluster",
						},
					},
					Addons: v1.ClusterAddonSpec{
						Source: v1.SourceSpec{
							Type: "local",
							URL:  "../config/samples/addons",
						},
						Jobs: []string{"cluster/local/minikube/all.yaml"},
					},
				},
				{
					Name: "second",
					Infra: v1.ClusterInfraSpec{
						X: map[string]string{
							"notOverridden": "default",
							"overridden":    "second-cluster",
						},
					},
					Addons: v1.ClusterAddonSpec{
						Source: v1.SourceSpec{
							Type: "local",
							URL:  "../config/samples/addons",
						},
						Jobs: []string{"cluster/local/minikube/all.yaml"},
					},
				},
			},
		},
	}
	for _, tst := range tests {
		t.Run(tst.it, func(t *testing.T) {
			got, err := flattenedClusterSpec(tst.in)
			assert.NoError(t, err)
			assert.Equal(t, tst.want, got)
		})
	}
}
