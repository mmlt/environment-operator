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
		EventSink:  &infoerFake{},
		Log:        testr.New(t),
	}
	stp := &step.InitStep{}
	ok, err := ex.Accept(stp)
	assert.NoError(t, err)
	assert.Equal(t, true, ok)

	//TODO wait for completion
}

// UpdaterFake records plan changes.
type updaterFake struct {
	steps []step.Step
}

func (u *updaterFake) Update(stp step.Step) {
	u.steps = append(u.steps, stp)
}

// InfoerFake records info and warning events.
type infoerFake struct {
	infos    []string
	warnings []string
}

func (in *infoerFake) Info(id step.ID, msg string) error {
	in.infos = append(in.infos, msg)
	return nil
}

func (in *infoerFake) Warning(id step.ID, msg string) error {
	in.warnings = append(in.warnings, msg)
	return nil
}
