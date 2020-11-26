package step

import (
	"context"
	"github.com/go-logr/logr"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"time"
)

// Step is the behaviour that all *Step types have in common.
type Step interface {
	// Meta returns a reference to the Metaa data of a Step.
	Meta() *Metaa
	// Execute a step, return true on success.
	Execute(context.Context, []string, Infoer, Updater, logr.Logger) bool
}

// Metaa contains the fields that all steps have in common.
// (it is embedded in all *Steps types)
type Metaa struct {
	// ID uniquely identifies this step.
	ID ID
	// Hash is unique for the config/parameters applied by this step.
	Hash string
	// State indicates if this step has started, is in error etc.
	State v1.StepState
	// Msg helps explaining the state. Mandatory for StepStateError.
	Msg string
	// LastUpdate is the time of the last state change.
	LastUpdate time.Time
}

// ID uniquely identifies a Step.
type ID struct {
	// Type is the type of step, for example; init, plan, apply.
	Type Type
	// Namespace Name identifies the plan to which the step belongs.
	Namespace, Name string
	// ClusterName (optional) is the name of the target cluster.
	ClusterName string
}

// ShortName returns a name that's unique within an environment.
func (si *ID) ShortName() string {
	return string(si.Type) + si.ClusterName
}

// Type of step.
type Type string

const (
	TypeInfra             Type = "Infra"
	TypeDestroy           Type = "Destroy"
	TypeAKSPool           Type = "AKSPool"
	TypeKubeconfig        Type = "Kubeconfig"
	TypeAKSAddonPreflight Type = "AKSAddonPreflight"
	TypeAddons            Type = "Addons"
)

// Updater is a third party that wants to know about Step state changes.
type Updater interface {
	Update(Step)
}

// Infoer is a third party that wants to receive info/warning events from a Step.
// The main purpose is to help the user understand/debug the system.
type Infoer interface {
	Info(id ID, msg string) error
	Warning(id ID, msg string) error
}
