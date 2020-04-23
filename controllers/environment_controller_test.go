package controllers

import (
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestController_processSpec(t *testing.T) {
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
					Infrastructure: v1.InfrastructureSpec{
						Source: v1.SourceSpec{
							Type: "local",
							URL:  "../config/samples/terraform", // relative to dir containing this _test.go file.
						},
						Main: "main.tf.tmplt",
						X: map[string]string{
							"overridden":    "cpe-cluster",
							"notOverridden": "default",
						},
					},
				}, {
					Name: "second",
					Infrastructure: v1.InfrastructureSpec{
						Source: v1.SourceSpec{
							Type: "local",
							URL:  "../config/samples/terraform", // relative to dir containing this _test.go file.
						},
						Main: "main.tf.tmplt",
						X: map[string]string{
							"overridden":    "second-cluster",
							"notOverridden": "default",
						},
					},
				},
			},
		},
	}
	for _, tst := range tests {
		t.Run(tst.it, func(t *testing.T) {
			got, err := processSpec(tst.in)
			assert.NoError(t, err)
			assert.Equal(t, tst.want, got)
		})
	}
}
