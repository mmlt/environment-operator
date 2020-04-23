package infra

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/mmlt/environment-operator/pkg/terraform"
	"time"
)

// Step is the behaviour that all *Step types have in common.
type Step interface {
	// Meta returns a reference to the meta data of a Step.
	Meta() *StepMeta
	// Type returns the type of a Step.
	Type() string
	// ID returns a unique identification of a Step.
	// TODO consider removing this method (use Meta() instead)
	id() StepID
	// Ord returns the execution order of a Step type.
	// (this method is a reminder to keep StepOrd const in sync with implementations)
	// TODO consider moving this to Plan
	ord() StepOrd
	// Execute a step, return true on success.
	execute(context.Context, Infoer, Updater, terraform.Terraformer, logr.Logger) bool
}

// StepMeta provides the fields common to all step types.
// It is embedded in all *Steps types.
type StepMeta struct {
	// ID uniquely identifies this step.
	ID StepID
	// State indicates if this step has started, is in error etc.
	State StepState
	// Msg helps explaining the state. Mandatory for StepStateError.
	Msg string
	// LastUpdate is the time of the last state change.
	LastUpdate time.Time
}

// StepID uniquely identifies a Step.
type StepID struct {
	// Type is the type of step, for example; init, plan, apply.
	Type string
	// Namespace Name identifies the plan to which the step belongs.
	Namespace, Name string
	// ClusterName (optional) is the name of the target cluster.
	ClusterName string
}

// StepState is the execute state of a step.
type StepState int

const (
	StepStateUnknown StepState = iota
	// StepMeta is in flight.
	StepStateRunning
	// StepMeta has completed successfully.
	StepStateReady
	// StepMeta has ended with an error.
	StepStateError
)

//go:generate stringer -type StepOrd -trimprefix StepOrd

// StepOrd provides the ordering in which steps should be executed.
// TODO remove or move to plan package
type StepOrd int

const (
	StepOrdTmplt StepOrd = iota
	StepOrdInit
	StepOrdPlan
	StepOrdApply
	StepOrdAddons
	StepOrdTest
	StepOrdLast // there is no LastStep
)
