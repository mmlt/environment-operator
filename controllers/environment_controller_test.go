package controllers

import (
	"errors"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
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

func Test_inSchedule(t *testing.T) {
	type args struct {
		schedule string
		now      string
	}
	tests := []struct {
		it   string
		args args
		want bool
		err  error
	}{
		{
			it: "should return true when in schedule",
			args: args{
				schedule: "* 15 * * *",
				now:      "2006-01-02T15:04:05Z",
			},
			want: true,
			err:  nil,
		},
		{
			it: "should return false when outside schedule",
			args: args{
				schedule: "* 22-23,0-4 * * *",
				now:      "2006-01-02T15:04:05Z",
			},
			want: false,
			err:  nil,
		},
		{
			it: "should return true at start of nightly schedule",
			args: args{
				schedule: "* 0-4 * * *",
				now:      "2006-01-02T00:04:05Z",
			},
			want: true,
			err:  nil,
		},
		{
			it: "should return and error on invalid schedule",
			args: args{
				schedule: "* 22-04 * * *",
				now:      "2006-01-03T03:04:05Z",
			},
			want: false,
			err:  errors.New("beginning of range (22) beyond end of range (4): 22-04"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.it, func(t *testing.T) {
			now, err := time.Parse(time.RFC3339, tt.args.now)
			assert.NoError(t, err)
			got, err := inSchedule(tt.args.schedule, now)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.err, err)
		})
	}
}
