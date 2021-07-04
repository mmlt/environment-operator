package controllers

import (
	"errors"
	"github.com/go-logr/stdr"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/step"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"log"
	"os"
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
							URL:  "testdata/addons",
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
							URL:  "testdata/addons",
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

func Test_syncStatusWithPlan(t *testing.T) {
	// test helper
	newStep := func(typ step.Type, clusterName, hash string) step.Step {
		return &step.AddonStep{ // we don't execute the step so we can use the same struct for this test
			Metaa: step.Metaa{
				ID: step.ID{
					Type:        typ,
					Namespace:   "ns",
					Name:        "name",
					ClusterName: clusterName,
				},
				Hash: hash,
			},
		}
	}
	newTime := func(t int64) metav1.Time {
		return metav1.Time{Time: time.Unix(t, 0)}
	}
	// fake time
	orgTimeNow := timeNow
	setTime := func(t int64) {
		timeNow = func() time.Time { return time.Unix(t, 0) }
	}

	// test cases
	type args struct {
		status v1.EnvironmentStatus
		plan   []step.Step
	}
	tests := []struct {
		it         string
		args       args
		wantStatus v1.EnvironmentStatus
		wantStep   step.Step
		wantErr    bool
	}{
		{
			it: "should return the first step of the plan when no steps have executed before",
			args: args{
				status: v1.EnvironmentStatus{},
				plan: []step.Step{
					newStep(step.TypeInfra, "", "123"),
					newStep(step.TypeAddons, "foo", "123"),
				}},
			wantStatus: v1.EnvironmentStatus{
				Steps: map[string]v1.StepStatus{
					"Infra":     {LastTransitionTime: newTime(0), State: "", Message: "new", Hash: ""},
					"Addonsfoo": {LastTransitionTime: newTime(0), State: "", Message: "new", Hash: ""},
				}},
			wantStep: newStep(step.TypeInfra, "", "123"),
			wantErr:  false,
		},
		{
			it: "should return the same first step of the plan when the step is executing",
			args: args{
				status: v1.EnvironmentStatus{
					Steps: map[string]v1.StepStatus{
						"Infra":     {LastTransitionTime: newTime(0), State: "Running", Message: "new", Hash: ""},
						"Addonsfoo": {LastTransitionTime: newTime(0), State: "", Message: "new", Hash: ""},
					}},
				plan: []step.Step{
					newStep(step.TypeInfra, "", "123"),
					newStep(step.TypeAddons, "foo", "456"),
				}},
			wantStatus: v1.EnvironmentStatus{
				Steps: map[string]v1.StepStatus{
					"Infra":     {LastTransitionTime: newTime(0), State: "Running", Message: "new", Hash: ""},
					"Addonsfoo": {LastTransitionTime: newTime(0), State: "", Message: "new", Hash: ""},
				}},
			wantStep: newStep(step.TypeInfra, "", "123"),
			wantErr:  false,
		},
		{
			it: "should return the second step of the plan when the first step has completed successfully (hashes match)",
			args: args{
				status: v1.EnvironmentStatus{
					Steps: map[string]v1.StepStatus{
						"Infra":     {LastTransitionTime: newTime(0), State: "Ready", Message: "new", Hash: "123"},
						"Addonsfoo": {LastTransitionTime: newTime(0), State: "", Message: "new", Hash: ""},
					}},
				plan: []step.Step{
					newStep(step.TypeInfra, "", "123"),
					newStep(step.TypeAddons, "foo", "456"),
				}},
			wantStatus: v1.EnvironmentStatus{
				Steps: map[string]v1.StepStatus{
					"Infra":     {LastTransitionTime: newTime(0), State: "Ready", Message: "new", Hash: "123"},
					"Addonsfoo": {LastTransitionTime: newTime(0), State: "", Message: "new", Hash: ""},
				}},
			wantStep: newStep(step.TypeAddons, "foo", "456"),
			wantErr:  false,
		},
		{
			it: "should return nil when all step have completed (hashes match)",
			args: args{
				status: v1.EnvironmentStatus{
					Steps: map[string]v1.StepStatus{
						"Infra":     {LastTransitionTime: newTime(0), State: "Ready", Message: "new", Hash: "123"},
						"Addonsfoo": {LastTransitionTime: newTime(0), State: "Ready", Message: "new", Hash: "456"},
					}},
				plan: []step.Step{
					newStep(step.TypeInfra, "", "123"),
					newStep(step.TypeAddons, "foo", "456"),
				}},
			wantStatus: v1.EnvironmentStatus{
				Steps: map[string]v1.StepStatus{
					"Infra":     {LastTransitionTime: newTime(0), State: "Ready", Message: "new", Hash: "123"},
					"Addonsfoo": {LastTransitionTime: newTime(0), State: "Ready", Message: "new", Hash: "456"},
				}},
			wantStep: nil,
			wantErr:  false,
		},
		{
			it: "should return the first step and clear states when hashes change",
			args: args{
				status: v1.EnvironmentStatus{
					Steps: map[string]v1.StepStatus{
						"Infra":     {LastTransitionTime: newTime(0), State: "Ready", Message: "new", Hash: "123"},
						"Addonsfoo": {LastTransitionTime: newTime(0), State: "Ready", Message: "new", Hash: "456"},
					}},
				plan: []step.Step{
					newStep(step.TypeInfra, "", "999123"),
					newStep(step.TypeAddons, "foo", "999456"),
				}},
			wantStatus: v1.EnvironmentStatus{
				Steps: map[string]v1.StepStatus{
					"Infra":     {LastTransitionTime: newTime(0), State: "", Message: "new", Hash: "123"},
					"Addonsfoo": {LastTransitionTime: newTime(0), State: "", Message: "new", Hash: "456"},
				}},
			wantStep: newStep(step.TypeInfra, "", "999123"),
			wantErr:  false,
		},
	}

	l := stdr.New(log.New(os.Stdout, "", log.Lshortfile|log.Ltime))

	setTime(0)
	for _, tt := range tests {
		t.Run(tt.it, func(t *testing.T) {
			status := tt.args.status.DeepCopy()
			gotStep, err := getStepAndSyncStatusWithPlan(status, tt.args.plan, l)
			if assert.NoError(t, err) {
				assert.Equal(t, tt.wantStep, gotStep)
				assert.Equal(t, &tt.wantStatus, status)
			}
		})
	}

	timeNow = orgTimeNow
}

func Test_updateStatusConditions(t *testing.T) {
	var (
		time1 = metav1.Time{Time: time.Date(2000, 1, 1, 1, 1, 1, 0, time.UTC)}
	)

	type args struct {
		status *v1.EnvironmentStatus
	}
	tests := []struct {
		it            string
		args          args
		wantCondition v1.EnvironmentCondition
	}{
		{
			it: "should say status: Unknown when all steps states are unknown (empty)",
			args: args{
				status: &v1.EnvironmentStatus{
					Steps: map[string]v1.StepStatus{
						"Infra":     {State: "", Message: "new", Hash: "123"},
						"Addonsfoo": {State: "", Message: "new", Hash: "456"},
					}},
			},
			wantCondition: v1.EnvironmentCondition{Type: "Ready", Status: "Unknown", Reason: "", Message: "0/2 ready, 0 running, 0 error(s)", LastTransitionTime: time1},
		},
		{
			it: "should say status: False reason: Running when some step(s) are running",
			args: args{
				status: &v1.EnvironmentStatus{
					Steps: map[string]v1.StepStatus{
						"Infra":     {State: "Running", Message: "new", Hash: "123"},
						"Addonsfoo": {State: "", Message: "new", Hash: "456"},
					}},
			},
			wantCondition: v1.EnvironmentCondition{Type: "Ready", Status: "False", Reason: "Running", Message: "0/2 ready, 1 running, 0 error(s)", LastTransitionTime: time1},
		},
		{
			it: "should say status: False reason: Running when some step(s) are ready and some are running",
			args: args{
				status: &v1.EnvironmentStatus{
					Steps: map[string]v1.StepStatus{
						"Infra":     {State: "Ready", Message: "new", Hash: "123"},
						"Addonsfoo": {State: "Running", Message: "new", Hash: "456"},
					}},
			},
			wantCondition: v1.EnvironmentCondition{Type: "Ready", Status: "False", Reason: "Running", Message: "1/2 ready, 1 running, 0 error(s)", LastTransitionTime: time1},
		},
		{
			it: "should say status: True reason: Failed when some step(s) are in error state",
			args: args{
				status: &v1.EnvironmentStatus{
					Steps: map[string]v1.StepStatus{
						"Infra":     {State: "Error", Message: "new", Hash: "123"},
						"Addonsfoo": {State: "", Message: "new", Hash: "456"},
					}},
			},
			wantCondition: v1.EnvironmentCondition{Type: "Ready", Status: "True", Reason: "Failed", Message: "0/2 ready, 0 running, 1 error(s)", LastTransitionTime: time1},
		},
		{
			it: "should say status: True, reason: Ready when all steps completed successfully",
			args: args{
				status: &v1.EnvironmentStatus{
					Steps: map[string]v1.StepStatus{
						"Infra":     {State: "Ready", Message: "new", Hash: "123"},
						"Addonsfoo": {State: "Ready", Message: "new", Hash: "456"},
					}},
			},
			wantCondition: v1.EnvironmentCondition{Type: "Ready", Status: "True", Reason: "Ready", Message: "2/2 ready, 0 running, 0 error(s)", LastTransitionTime: time1},
		},
		{
			it: "should say status: Unknown, reason: empty when no steps have been defined",
			args: args{
				status: &v1.EnvironmentStatus{},
			},
			wantCondition: v1.EnvironmentCondition{Type: "Ready", Status: "Unknown", Reason: "", Message: "0/0 ready, 0 running, 0 error(s)", LastTransitionTime: time1},
		},
	}

	// fake time
	tn := timeNow
	defer func() { timeNow = tn }()
	timeNow = func() time.Time { return time1.Time }

	for _, tt := range tests {
		t.Run(tt.it, func(t *testing.T) {
			status := tt.args.status.DeepCopy()
			updateStatusConditions(status)
			if assert.Equal(t, 1, len(status.Conditions)) {
				assert.Equal(t, tt.wantCondition, status.Conditions[0])
			}
		})
	}

}
