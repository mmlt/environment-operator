package plan

import (
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestConditions_After(t *testing.T) {
	data := &conditions{
		inner: []v1.EnvironmentCondition{
			{Type: "new", LastTransitionTime: testToTime(3)},
			{Type: "older", LastTransitionTime: testToTime(2)},
			{Type: "oldest", LastTransitionTime: testToTime(1)},
		},
	}

	tsts := []struct {
		in   []string
		want bool
	}{
		{in: []string{"new", "older", "oldest"}, want: true},
		{in: []string{"new", "oldest"}, want: true},
		{in: []string{"oldest"}, want: true},
		{in: []string{"oldest", "older"}, want: false},
		{in: []string{"oldest", "older", "oldest"}, want: false},
	}

	for _, tst := range tsts {
		got := data.after(tst.in...)
		assert.Equal(t, tst.want, got, tst.in)
	}
}
