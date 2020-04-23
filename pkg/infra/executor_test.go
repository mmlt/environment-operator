package infra

import (
	"github.com/mmlt/environment-operator/pkg/terraform"
	"github.com/mmlt/testr"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestExecutor_Accept(t *testing.T) {
	updates := &updaterFake{}
	infos := &infoerFake{}
	tf := &terraform.TerraformFake{Log: testr.New(t).WithName("tf")}
	tf.SetupFakeResults()

	ex := NewExecutor(updates, infos, tf, testr.New(t))

	plan := planWithSteps("init")
	ok, err := ex.Accept(plan)
	assert.NoError(t, err)
	assert.Equal(t, true, ok)

	//TODO wait for assert
}

// PlanWithSteps returns a Plan for testing.
func planWithSteps(names ...string) Plan {
	p := Plan{
		Namespace: "default",
		Name:      "xyz",
	}

	for _, n := range names {
		var st Step
		switch n {
		/*		case "source":
					st = &SourceStep{
						StepMeta: StepMeta{},
					}
				case "init":
					st = &InitStep{
						StepMeta: StepMeta{},
					}
				case "plan":
					st = &PlanStep{
						StepMeta: StepMeta{},
					}*/
		case "apply":
			st = &ApplyStep{
				StepMeta: StepMeta{},
			}

		default:
			panic("unknown step name: " + n)
		}
		p.Steps = append(p.Steps, st)
	}
	return p
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
