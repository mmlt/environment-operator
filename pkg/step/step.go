package step

import (
	"context"
	"github.com/go-logr/logr"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/terraform"
	"time"
)

// Step is the behaviour that all *Step types have in common.
type Step interface {
	// Meta returns a reference to the meta data of a Step.
	Meta() *StepMeta
	//// Type returns the type of a Step. //TODO use Meta().State.Type?
	//Type() string
	// ID returns a unique identification of a Step.
	// TODO consider removing this method (use Meta() instead)
	//id() StepID
	// Ord returns the execution order of a Step type.
	// (this method is a reminder to keep StepOrd const in sync with implementations)
	// TODO consider moving this to Plan
	//ord() StepOrd
	// Execute a step, return true on success.
	Execute(context.Context, Infoer, Updater, terraform.Terraformer, logr.Logger) bool //TODO move terraformer to StepInit,Plan,Apply structs (planner is responsible for setting)
}

// StepMeta provides the fields common to all step types.
// It is embedded in all *Steps types.
type StepMeta struct {
	// ID uniquely identifies this step.
	ID StepID
	// Hash is unique for the config/parameters applied by this step.
	Hash string
	// State indicates if this step has started, is in error etc.
	State v1.StepStatusState //TODO StepState
	// Msg helps explaining the state. Mandatory for StepStateError.
	Msg string
	// LastUpdate is the time of the last state change.
	LastUpdate time.Time
}

// StepID uniquely identifies a Step.
type StepID struct {
	// Type is the type of step, for example; init, plan, apply.
	Type StepType
	// Namespace Name identifies the plan to which the step belongs.
	Namespace, Name string
	// ClusterName (optional) is the name of the target cluster.
	ClusterName string
}

// ShortName returns a name that's unique within an environment.
func (si *StepID) ShortName() string {
	return si.Type.String() + si.ClusterName
}

//go:generate stringer -type StepType -trimprefix StepType

// StepType allows us to iterate step types. //TODO do we need iteration? why not use const StepTypeInit = "Init" and remove go:generate?
type StepType int

const (
	//TODO StepTypeLogin
	StepTypeInit StepType = iota
	StepTypePlan
	StepTypeApply
	StepTypePool
	StepTypeKubeconfig
	StepTypeAddons
	StepTypeTest
	StepTypeLast // there is no LastStep
)

// Updater is a third party that wants to know about Step changes.
type Updater interface {
	Update(Step)
}

// Infoer is a third party that wants to receive info/warning events from a Step.
// The main purpose is to help the user understand/debug the system.
type Infoer interface {
	Info(id StepID, msg string) error
	Warning(id StepID, msg string) error
}
