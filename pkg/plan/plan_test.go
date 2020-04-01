package plan

import (
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetTFState(t *testing.T) {
	// It can read an empty state.
	plan := New(&v1.Environment{})
	got, err := plan.GetTFState()
	assert.NoError(t, err)
	assert.Empty(t, got)
}

func TestSetTFState(t *testing.T) {
	// It can write a state and read it back.
	const want = "the quick brown fox jumped over the lazy dog"
	plan := New(&v1.Environment{})
	err := plan.PutTFState([]byte(want))
	assert.NoError(t, err)

	got, err := plan.GetTFState()
	assert.NoError(t, err)
	assert.Equal(t, want, string(got))
}
