package plan

import (
	"fmt"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/infra"
	"github.com/mmlt/environment-operator/pkg/source"
	"github.com/mmlt/testr"
	"github.com/stretchr/testify/assert"
	"hash"
	"hash/fnv"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"testing"
	"time"
)

// Throughout the code the terms day1 and day2 are used meaning;
//   day1: no previous state (conditions) are present.
//   day2: state from previous run is present.
//
// To represent time the tests use 3 digits of the year field formatted 'DSS' with D=Day 1 or 2 and SS as a step number.

//TODO It should return no step when there is no infra, cluster or test change
//TODO Test for Day1 and Day2

// Infra hash has changed.
func TestNextStep_InfraChanged(t *testing.T) {
	tests := []struct {
		it     string
		src    fakeSource
		spec   []v1.ClusterSpec
		status v1.EnvironmentStatus
		want   infra.Step
	}{
		// It should return no step when any step is running.
		{
			it:  "should_return_no_step_when_an_InitStep_is_running",
			src: fakeSource{source.Ninfra: {"path/to/infra/src", toHash(12)}},
			status: v1.EnvironmentStatus{
				Conditions: []v1.EnvironmentCondition{
					{Type: "InfraInit", Status: metav1.ConditionTrue, Reason: v1.ReasonRunning, LastTransitionTime: toTime(100)},
					{Type: "InfraPlan", Status: metav1.ConditionFalse, Reason: v1.ReasonReady, LastTransitionTime: toTime(101)},
					{Type: "InfraApply", Status: metav1.ConditionFalse, Reason: v1.ReasonReady, LastTransitionTime: toTime(102)},
				},
			},
			want: nil,
		}, {
			it:  "should_return_no_step_when_a_PlanStep_is_running",
			src: fakeSource{source.Ninfra: {"path/to/infra/src", toHash(12)}},
			status: v1.EnvironmentStatus{
				Conditions: []v1.EnvironmentCondition{
					{Type: "InfraInit", Status: metav1.ConditionFalse, Reason: v1.ReasonReady, LastTransitionTime: toTime(100)},
					{Type: "InfraPlan", Status: metav1.ConditionTrue, Reason: v1.ReasonRunning, LastTransitionTime: toTime(101)},
					{Type: "InfraApply", Status: metav1.ConditionFalse, Reason: v1.ReasonReady, LastTransitionTime: toTime(102)},
				},
			},
			want: nil,
		}, {
			it:  "should_return_no_step_when_an_ApplyStep_is_running",
			src: fakeSource{source.Ninfra: {"path/to/infra/src", toHash(12)}},
			status: v1.EnvironmentStatus{
				Conditions: []v1.EnvironmentCondition{
					{Type: "InfraInit", Status: metav1.ConditionFalse, Reason: v1.ReasonReady, LastTransitionTime: toTime(100)},
					{Type: "InfraPlan", Status: metav1.ConditionFalse, Reason: v1.ReasonReady, LastTransitionTime: toTime(101)},
					{Type: "InfraApply", Status: metav1.ConditionTrue, Reason: v1.ReasonRunning, LastTransitionTime: toTime(102)},
				},
			},
			want: nil,
		},

		// It should return a next step.
		{
			it:  "should_return_an_InitStep_on_day1",
			src: fakeSource{source.Ninfra: {"path/to/infra/src", toHash(12)}},
			spec: []v1.ClusterSpec{{}},  // at least one cluster because it contains infra values.
			want: &infra.InitStep{
				SourcePath: "path/to/infra/src", Hash: toHashString(12),
			},
		}, {
			it:  "should_return_a_InitStep_on_day2",
			src: fakeSource{source.Ninfra: {"path/to/infra/src", toHash(12)}},
			spec: []v1.ClusterSpec{{}},
			status: v1.EnvironmentStatus{
				Conditions: []v1.EnvironmentCondition{
					{Type: "InfraInit", Status: metav1.ConditionFalse, Reason: v1.ReasonReady, LastTransitionTime: toTime(100)},
					{Type: "InfraPlan", Status: metav1.ConditionFalse, Reason: v1.ReasonReady, LastTransitionTime: toTime(101)},
					{Type: "InfraApply", Status: metav1.ConditionFalse, Reason: v1.ReasonReady, LastTransitionTime: toTime(102)},
				},
			},
			want: &infra.InitStep{
				SourcePath: "path/to/infra/src", Hash: toHashString(12),
			},
		}, {
			it:  "should_return_a_PlanStep_when_an_InitStep_completed_successfully_(day1)",
			src: fakeSource{source.Ninfra: {"path/to/infra/src", toHash(12)}},
			status: v1.EnvironmentStatus{
				Conditions: []v1.EnvironmentCondition{
					{Type: "InfraInit", Status: metav1.ConditionFalse, Reason: v1.ReasonReady, LastTransitionTime: toTime(100)},
				},
			},
			want: &infra.PlanStep{
				SourcePath: "path/to/infra/src",
			},
		}, {
			it:  "should_return_a_PlanStep_when_an_InitStep_completed_successfully_(day2)",
			src: fakeSource{source.Ninfra: {"path/to/infra/src", toHash(12)}},
			status: v1.EnvironmentStatus{
				Conditions: []v1.EnvironmentCondition{
					{Type: "InfraInit", Status: metav1.ConditionFalse, Reason: v1.ReasonReady, LastTransitionTime: toTime(200)},
					{Type: "InfraPlan", Status: metav1.ConditionFalse, Reason: v1.ReasonReady, LastTransitionTime: toTime(101)},
					{Type: "InfraApply", Status: metav1.ConditionFalse, Reason: v1.ReasonReady, LastTransitionTime: toTime(102)},
				},
			},
			want: &infra.PlanStep{
				SourcePath: "path/to/infra/src",
			},
		}, {
			it:  "should_return_an_ApplyStep_when_a_PlanStep_completed_successfully_(day1)",
			src: fakeSource{source.Ninfra: {"path/to/infra/src", toHash(12)}},
			status: v1.EnvironmentStatus{
				Conditions: []v1.EnvironmentCondition{
					{Type: "InfraInit", Status: metav1.ConditionFalse, Reason: v1.ReasonReady, LastTransitionTime: toTime(200)},
					{Type: "InfraPlan", Status: metav1.ConditionFalse, Reason: v1.ReasonReady, LastTransitionTime: toTime(201)},
					{Type: "InfraApply", Status: metav1.ConditionFalse, Reason: v1.ReasonReady, LastTransitionTime: toTime(102)},
				},
			},
			want: &infra.ApplyStep{
				SourcePath: "path/to/infra/src", Hash: toHashString(12), Added: 0, Changed: 0, Deleted: 0,
			},
		}, {
			it:  "should_return_an_ApplyStep_when_a_PlanStep_completed_successfully_(day2)",
			src: fakeSource{source.Ninfra: {"path/to/infra/src", toHash(12)}},
			status: v1.EnvironmentStatus{
				Conditions: []v1.EnvironmentCondition{
					{Type: "InfraInit", Status: metav1.ConditionFalse, Reason: v1.ReasonReady, LastTransitionTime: toTime(100)},
					{Type: "InfraPlan", Status: metav1.ConditionFalse, Reason: v1.ReasonReady, LastTransitionTime: toTime(101)},
				},
			},
			want: &infra.ApplyStep{
				SourcePath: "path/to/infra/src", Hash: toHashString(12), Added: 0, Changed: 0, Deleted: 0,
			},
		},
	}

	plan := Plan{
		Log: testr.New(t),
	}
	nsn := types.NamespacedName{Namespace: "default", Name: "env-for-testing"}

	for _, tst := range tests {
		t.Run(tst.it, func(t *testing.T) {
			got, err := plan.nextStep(nsn, tst.src, tst.spec, tst.status)
			assert.NoError(t, err)
			assert.Equal(t, tst.want, got)
		})
	}
}

type fakeSource map[string]struct {
	dir  string
	hash hash.Hash
}
// FakeSource implements source.Getter
var _ source.Getter = fakeSource{}

func (fs fakeSource) Hash(nsn types.NamespacedName, name string) (hash.Hash, error) {
	v, ok := fs[name]
	if !ok {
		return nil, fmt.Errorf("source not found: %s", name)
	}
	return v.hash, nil
}

func (fs fakeSource) Get(nsn types.NamespacedName, name string) (string, hash.Hash, error) {
	v, ok := fs[name]
	if !ok {
		return "", nil, fmt.Errorf("source not found: %s", name)
	}
	return v.dir, v.hash, nil
}

// ToHash returns a hash for testing.
func toHash(b byte) hash.Hash {
	h := fnv.New64()
	h.Write([]byte{b})
	return h
}

// ToHashString is toHash with string output.
func toHashString(b byte) string {
	h := toHash(b)
	return hashAsString(h)
}

// ToTime returns a time for testing.
func toTime(n int) metav1.Time {
	t := time.Date(n, 1, 1, 0, 0, 0, 0, time.UTC)
	return metav1.Time{Time: t}
}

// ArrayAsMap accepts an array with key, value strings and returns a map.
func arrayAsMap(in []string) map[string]string {
	r := map[string]string{}

	if len(in)%2 != 0 {
		panic("in must be even length")
	}

	for i := 0; i < len(in); i = +2 {
		k, v := in[i], in[i+1]
		r[k] = v
	}

	return r
}
