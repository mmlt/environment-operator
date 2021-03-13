package executor

import (
	"github.com/mmlt/environment-operator/pkg/step"
	"github.com/mmlt/testr"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestExecutor_Accept(t *testing.T) {
	ex := Executor{
		UpdateSink: &updaterFake{},
		Log:        testr.New(t),
	}
	stp := &step.InfraStep{}
	ok, err := ex.Accept(stp)
	assert.NoError(t, err)
	assert.Equal(t, true, ok)
}

// UpdaterFake records plan changes.
type updaterFake struct {
	steps []step.Meta
}

func (u *updaterFake) Update(stp step.Meta) {
	u.steps = append(u.steps, stp)
}
