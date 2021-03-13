package step

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"strings"
	"sync"
	"time"
)

// Step is an unit of execution.
type Step interface {
	Meta
	// Execute a step.
	Execute(context.Context, []string, logr.Logger)
}

// Meta is behaviour that all steps have in common.
type Meta interface {
	GetID() ID
	GetHash() string
	GetState() v1.StepState
	GetMsg() string
	GetLastUpdate() time.Time
	GetLastError() error
	SetOnUpdate(fn MetaUpdateFn)
}

// Metaa is the data that all steps have in common.
// (it is embedded in all Steps types)
type Metaa struct {
	// ID uniquely identifies a step.
	ID ID
	// Hash is unique for the config/parameters applied by a step.
	Hash string
	// State indicates if a step is running, ready or is in error.
	State v1.StepState
	// Msg helps explaining the state. Mandatory for StepStateError.
	Msg string
	// LastUpdate is the time of the last state change.
	LastUpdate time.Time
	// LastError contains the last encountered error or nil.
	lastError error
	// OnUpdate (optional) is a function that is called after updating.
	onUpdate MetaUpdateFn
	// Mu is a mutex.
	mu sync.Mutex
}

type MetaUpdateFn func(Meta)

var _ Meta = &Metaa{}

func (m *Metaa) GetID() ID {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.ID
}

func (m *Metaa) GetHash() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.Hash
}

func (m *Metaa) GetState() v1.StepState {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.State
}

func (m *Metaa) GetMsg() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.Msg
}

func (m *Metaa) GetLastUpdate() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.LastUpdate
}

func (m *Metaa) GetLastError() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.lastError
}

func (m *Metaa) SetOnUpdate(fn MetaUpdateFn) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.onUpdate = fn
}

// Update updates Step meta and notifies on-update listeners.
func (m *Metaa) update(state v1.StepState, msg string) {
	m.mu.Lock()
	if !IsStateLE(m.State, state) {
		m.mu.Unlock()
		panic(fmt.Sprintf("Step %s state is regressing from %s to %s (bug in Execute()?)", m.ID.ShortName(), m.State, state))
	}
	m.State = state
	m.Msg = msg
	m.LastUpdate = time.Now()
	f := m.onUpdate
	m.mu.Unlock()

	if f != nil {
		f(m)
	}
}

// Error2 updates Step meta, sets Error state and notifies on-update listeners.
func (m *Metaa) error2(err error, msg string) {
	m.mu.Lock()
	m.lastError = err
	m.mu.Unlock()

	m.update(v1.StateError, msg)
}

// ID uniquely identifies a Step.
type ID struct {
	// Type is the type of step, for example; Infra, Destroy, Addons.
	Type Type
	// Namespace Name identifies the plan to which the step belongs.
	Namespace, Name string
	// ClusterName (optional) is the name of the target cluster.
	ClusterName string
}

// ShortName returns a name that's unique within an environment.
func (si ID) ShortName() string {
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

// Types is an enumeration of all types.
var Types = []Type{TypeInfra, TypeDestroy, TypeAKSPool, TypeKubeconfig, TypeAKSAddonPreflight, TypeAddons}

// IsStateFinal returns true is state is a final state.
// A step in final state has stopped executing.
func IsStateFinal(state v1.StepState) bool {
	return state == v1.StateReady || state == v1.StateError
}

// IsStateLE returns true if lhs is less or equal to rhs assuming the ordering; "", Running, Ready | Error
func IsStateLE(lhs, rhs v1.StepState) bool {
	toNum := func(s v1.StepState) int {
		switch s {
		case "":
			return 0
		case v1.StateRunning:
			return 1
		case v1.StateReady, v1.StateError:
			return 2
		default:
			panic("bug: state missing")
			return 99
		}
	}
	return toNum(lhs) <= toNum(rhs)
}

// Updater is a third party that wants to know about Step state changes.
type Updater interface {
	Update(Meta)
}

// TypeFromString converts a comma separated list of type names to a set of Type.
// On empty input an empty set is returned.
func TypesFromString(s string) (map[Type]struct{}, error) {
	r := make(map[Type]struct{})

	if s == "" {
		return r, nil
	}

	valid := make(map[string]struct{}, len(Types))
	for _, v := range Types {
		valid[string(v)] = struct{}{}
	}

	var e []string
	for _, v := range strings.Split(s, ",") {
		if _, ok := valid[v]; ok {
			r[Type(v)] = struct{}{}
		} else {
			e = append(e, v)
		}
	}

	var err error
	if len(e) > 0 {
		err = fmt.Errorf("unknown step type(s): %v", e)
	}

	return r, err
}
