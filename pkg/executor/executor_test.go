package executor

import (
	"github.com/mmlt/environment-operator/pkg/terraform"
	"github.com/mmlt/testr"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestExecutor_Accept(t *testing.T) {
	tf := &terraform.TerraformFake{Log: testr.New(t).WithName("tf")}
	tf.SetupFakeResults()
	ex := Executor{
		UpdateSink: &updaterFake{},
		EventSink:  &infoerFake{},
		Terraform:  tf,
		Log:        testr.New(t),
	}
	step := &InitStep{}
	ok, err := ex.Accept(step)
	assert.NoError(t, err)
	assert.Equal(t, true, ok)

	//TODO wait for completion
}

// UpdaterFake records plan changes.
type updaterFake struct {
	steps []Step
}

func (u *updaterFake) Update(step Step) {
	u.steps = append(u.steps, step)
}

// InfoerFake records info and warning events.
type infoerFake struct {
	infos    []string
	warnings []string
}

func (in *infoerFake) Info(id StepID, msg string) error {
	in.infos = append(in.infos, msg)
	return nil
}

func (in *infoerFake) Warning(id StepID, msg string) error {
	in.warnings = append(in.warnings, msg)
	return nil
}
