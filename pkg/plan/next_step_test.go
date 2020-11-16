package plan

import (
	"fmt"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/source"
	"github.com/mmlt/environment-operator/pkg/step"
	"github.com/mmlt/testr"
	"github.com/stretchr/testify/assert"
	"hash"
	"hash/fnv"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"testing"
	"time"
)

// TODO bring up-to-date or rewrite these tests.

// Throughout the code the terms day1 and day2 are used meaning;
//   day1: no previous state (conditions) are present.
//   day2: state from previous run is present.
//
// To represent time the tests use 3 digits of the year field formatted 'DSS' with D=Day 1 or 2 and SS as a step number.

// infra is the name for infra related sources.
const infra = ""

// Infra hash has changed.
func TestNextStep_InfraChanged(t *testing.T) {
	tests := []struct {
		it     string
		src    fakeSource
		ispec  v1.InfraSpec
		cspec  []v1.ClusterSpec
		status v1.EnvironmentStatus
		want   step.Step
	}{
		// It should return no step when any step is running.
		{
			it:  "should_return_no_step_when_an_InitStep_is_running",
			src: fakeSource{infra: {"path/to/infra/src", "hash1"}},
			status: v1.EnvironmentStatus{
				Conditions: []v1.EnvironmentCondition{
					{Type: "InfraInit", Status: metav1.ConditionTrue, Reason: v1.ReasonRunning, LastTransitionTime: testToTime(100)},
					{Type: "InfraPlan", Status: metav1.ConditionFalse, Reason: v1.ReasonReady, LastTransitionTime: testToTime(101)},
					{Type: "InfraApply", Status: metav1.ConditionFalse, Reason: v1.ReasonReady, LastTransitionTime: testToTime(102)},
				},
			},
			want: nil,
		}, {
			it:  "should_return_no_step_when_a_PlanStep_is_running",
			src: fakeSource{infra: {"path/to/infra/src", "hash1"}},
			status: v1.EnvironmentStatus{
				Conditions: []v1.EnvironmentCondition{
					{Type: "InfraInit", Status: metav1.ConditionFalse, Reason: v1.ReasonReady, LastTransitionTime: testToTime(100)},
					{Type: "InfraPlan", Status: metav1.ConditionTrue, Reason: v1.ReasonRunning, LastTransitionTime: testToTime(101)},
					{Type: "InfraApply", Status: metav1.ConditionFalse, Reason: v1.ReasonReady, LastTransitionTime: testToTime(102)},
				},
			},
			want: nil,
		}, {
			it:  "should_return_no_step_when_an_ApplyStep_is_running",
			src: fakeSource{infra: {"path/to/infra/src", "hash1"}},
			status: v1.EnvironmentStatus{
				Conditions: []v1.EnvironmentCondition{
					{Type: "InfraInit", Status: metav1.ConditionFalse, Reason: v1.ReasonReady, LastTransitionTime: testToTime(100)},
					{Type: "InfraPlan", Status: metav1.ConditionFalse, Reason: v1.ReasonReady, LastTransitionTime: testToTime(101)},
					{Type: "InfraApply", Status: metav1.ConditionTrue, Reason: v1.ReasonRunning, LastTransitionTime: testToTime(102)},
				},
			},
			want: nil,
		},

		// It should return a next step.
		{
			it:  "should_return_an_InitStep_on_day1",
			src: fakeSource{infra: {"path/to/infra/src", "hash1"}},
			//TODO cspec: []v1.ClusterSpec{{}}, // at least one cluster because it contains infra values.
			want: &step.InitStep{
				SourcePath: "path/to/infra/src", Hash: "hash1",
			},
		}, {
			it:  "should_return_a_InitStep_on_day2",
			src: fakeSource{infra: {"path/to/infra/src", "hash1"}},
			//TODO cspec: []v1.ClusterSpec{{}},
			status: v1.EnvironmentStatus{
				Conditions: []v1.EnvironmentCondition{
					{Type: "InfraInit", Status: metav1.ConditionFalse, Reason: v1.ReasonReady, LastTransitionTime: testToTime(100)},
					{Type: "InfraPlan", Status: metav1.ConditionFalse, Reason: v1.ReasonReady, LastTransitionTime: testToTime(101)},
					{Type: "InfraApply", Status: metav1.ConditionFalse, Reason: v1.ReasonReady, LastTransitionTime: testToTime(102)},
				},
			},
			want: &step.InitStep{
				SourcePath: "path/to/infra/src", Hash: "hash1",
			},
		}, {
			it:  "should_return_a_PlanStep_when_an_InitStep_completed_successfully_(day1)",
			src: fakeSource{infra: {"path/to/infra/src", "hash1"}},
			status: v1.EnvironmentStatus{
				Conditions: []v1.EnvironmentCondition{
					{Type: "InfraInit", Status: metav1.ConditionFalse, Reason: v1.ReasonReady, LastTransitionTime: testToTime(100)},
				},
			},
			want: &step.PlanStep{
				SourcePath: "path/to/infra/src",
			},
		}, {
			it:  "should_return_a_PlanStep_when_an_InitStep_completed_successfully_(day2)",
			src: fakeSource{infra: {"path/to/infra/src", "hash1"}},
			status: v1.EnvironmentStatus{
				Conditions: []v1.EnvironmentCondition{
					{Type: "InfraInit", Status: metav1.ConditionFalse, Reason: v1.ReasonReady, LastTransitionTime: testToTime(200)},
					{Type: "InfraPlan", Status: metav1.ConditionFalse, Reason: v1.ReasonReady, LastTransitionTime: testToTime(101)},
					{Type: "InfraApply", Status: metav1.ConditionFalse, Reason: v1.ReasonReady, LastTransitionTime: testToTime(102)},
				},
			},
			want: &step.PlanStep{
				SourcePath: "path/to/infra/src",
			},
		}, {
			it:  "should_return_an_ApplyStep_when_a_PlanStep_completed_successfully_(day1)",
			src: fakeSource{infra: {"path/to/infra/src", "hash1"}},
			status: v1.EnvironmentStatus{
				Conditions: []v1.EnvironmentCondition{
					{Type: "InfraInit", Status: metav1.ConditionFalse, Reason: v1.ReasonReady, LastTransitionTime: testToTime(200)},
					{Type: "InfraPlan", Status: metav1.ConditionFalse, Reason: v1.ReasonReady, LastTransitionTime: testToTime(201)},
					{Type: "InfraApply", Status: metav1.ConditionFalse, Reason: v1.ReasonReady, LastTransitionTime: testToTime(102)},
				},
			},
			want: &step.ApplyStep{
				SourcePath: "path/to/infra/src", Hash: "hash1", Added: 0, Changed: 0, Deleted: 0,
			},
		}, {
			it:  "should_return_an_ApplyStep_when_a_PlanStep_completed_successfully_(day2)",
			src: fakeSource{infra: {"path/to/infra/src", "hash1"}},
			status: v1.EnvironmentStatus{
				Conditions: []v1.EnvironmentCondition{
					{Type: "InfraInit", Status: metav1.ConditionFalse, Reason: v1.ReasonReady, LastTransitionTime: testToTime(100)},
					{Type: "InfraPlan", Status: metav1.ConditionFalse, Reason: v1.ReasonReady, LastTransitionTime: testToTime(101)},
				},
			},
			want: &step.ApplyStep{
				SourcePath: "path/to/infra/src", Hash: "hash1", Added: 0, Changed: 0, Deleted: 0,
			},
		},
	}

	plan := Planner{
		Log: testr.New(t),
	}
	nsn := types.NamespacedName{Namespace: "default", Name: "env-for-testing"}

	for _, tst := range tests {
		t.Run(tst.it, func(t *testing.T) {
			got, err := plan.nextStep(nsn, tst.src, tst.ispec, tst.cspec, tst.status)
			assert.NoError(t, err)
			assert.Equal(t, tst.want, got)
		})
	}
}

type fakeSource map[string]struct {
	dir  string
	hash string
}

// FakeSource implements source.Sourcer
var _ source.Getter = fakeSource{}

func (fs fakeSource) Hash(nsn types.NamespacedName, name string) (string, error) {
	v, ok := fs[name]
	if !ok {
		return "", fmt.Errorf("source not found: %s", name)
	}
	return v.hash, nil
}

func (fs fakeSource) Get(nsn types.NamespacedName, name string) (string, error) {
	v, ok := fs[name]
	if !ok {
		return "", fmt.Errorf("source not found: %s", name)
	}
	return v.dir, nil
}

// ToHash returns a hash for testing.
func testToHash(b byte) hash.Hash {
	h := fnv.New64()
	h.Write([]byte{b})
	return h
}

// ToHashString is toHash with string output.
func testToHashString(b byte) string {
	return fmt.Sprint("%v", b)
}

// ToTime returns a time for testing.
func testToTime(n int) metav1.Time {
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
