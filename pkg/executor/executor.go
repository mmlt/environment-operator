// Package executor performs steps to create environments.
//
// Executor accepts steps, runs them and keeps track of them.
package executor

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/step"
	"github.com/mmlt/environment-operator/pkg/util"
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	"sync"
)

// Executor performs Steps.
type Executor struct {
	// UpdateSink is notified when step meta has changed.
	UpdateSink step.Updater

	// Environ are the environment variables presented to the process.
	Environ map[string]string

	// Running is the map of running steps.
	running map[step.ID]*run

	Log logr.Logger
	sync.Mutex
}

// Run is the concurrent execution of a step.
type run struct {
	// Ctx provides step cancellation.
	ctx context.Context
	// ExitedCh is closed when the worker has exited.
	exitedCh chan<- interface{}
	// Exited is true when the worker has exited.
	exited bool
	// Step is the step being executed.
	step step.Step
}

// MaxWorkers is the maximum number of workers available to run a step.
// For now set at 1 to prevent parallel execution of steps when an environment resource is deleted/created (erasing status)
// This number can be increased when the Planner not only takes environment.status but also executing steps into account
// when calculating next step.
const maxWorkers = 1

// Prometheus metrics.
var (
	MetricSteps = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "steps_total",
			Help: "Number of steps started",
		},
	)
	MetricStepRestarts = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "step_restarts_total",
			Help: "Number of steps restarted",
		},
	)
	MetricStepFailures = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "step_failures_total",
			Help: "Number of steps ending in error state",
		},
	)
)

func init() {
	// Register custom metrics with the global prometheus registry.
	metrics.Registry.MustRegister(MetricSteps, MetricStepFailures)
}

// Accept attempts to execute another step and returns true if step is accepted.
//
// It is ok (even desired) to keep calling Accept with the same step as long as the step is not in its final
// (Ready or Error) state. This will make sure a step is re-executed when it exits before reaching final state for
// example when the program is accidentally restarted.
// When a step is in its final state it must be Release'd before it can be executed again.
func (ex *Executor) Accept(stp step.Step) (bool, error) {
	if stp == nil {
		// nothing to do
		return true, nil
	}

	if stp.GetState() == v1.StateError {
		// should not run
		return false, nil
	}

	ex.Lock()
	defer ex.Unlock()

	if ex.running == nil {
		ex.running = make(map[step.ID]*run)
	}

	var ss v1.StepState
	var exited bool
	if r, ok := ex.running[stp.GetID()]; ok {
		ss = r.step.GetState()
		exited = r.exited
	}
	if step.IsStateFinal(ss) {
		// step is in final state (it needs to be released before it can be executed again)
		return true, nil
	}
	if ss == v1.StateRunning && !exited {
		// step is till running
		return true, nil
	}

	// max workers limit
	rt := 0
	for _, r := range ex.running {
		if !r.exited {
			rt++
		}
	}
	if rt >= maxWorkers {
		// max number of parallel runs reached.
		return false, nil
	}

	// step is new or is already in running state but has exited and needs to execute again.
	ex.Log.V(2).Info("Accept", "id", stp.GetID().ShortName(), "rerun", exited)

	if !(ss == "" || (ss == v1.StateRunning && exited)) {
		return false, fmt.Errorf("expect known 'StepState' new OR 'StepState' Running and 'exited' (got StepState=%s, exited=%v)", ss, exited)
	}

	var rn *run
	if exited {
		// a known step
		rn = ex.running[stp.GetID()]
		rn.exited = false
		MetricStepRestarts.Inc()
	} else {
		// a new step
		rn = &run{
			step: stp,
		}
		ex.running[stp.GetID()] = rn
		MetricSteps.Inc()
	}
	rn.ctx = context.Background()
	rn.exitedCh = make(chan<- interface{})

	go func(r *run) {
		log := ex.Log.WithName(stp.GetID().ShortName())

		stp.SetOnUpdate(func(meta step.Meta) {
			e := meta.GetLastError()
			m := meta.GetMsg()
			if e != nil {
				log.Error(e, m)
			}
			s := meta.GetState()
			if s == v1.StateError {
				MetricStepFailures.Inc()
			}
			log.Info("OnUpdate", "msg", m, "state", s, "id", meta.GetID().ShortName())
			ex.UpdateSink.Update(meta)
		})

		stp.Execute(r.ctx, ex.environSlice(), log)

		ex.Lock()
		r.exited = true
		ex.Unlock()

		close(r.exitedCh)
	}(rn)

	return true, nil
}

// Release removes a step with id from list of accepted steps.
// After a step is accepted and has run to its final state it must be released before it can be run again.
func (ex *Executor) Release(id step.ID) error {
	ex.Lock()
	defer ex.Unlock()

	rn, known := ex.running[id]
	if !known {
		return fmt.Errorf("release: %v not known", id)
	}

	if !step.IsStateFinal(rn.step.GetState()) {
		return fmt.Errorf("release: %v in %v", id, rn.step.GetState())
	}
	// it's ok to release when a step is final state but the worker go routine has not exited yet.

	ex.Log.V(2).Info("release", "id", id.ShortName())

	delete(ex.running, id)

	return nil
}

// ReleaseSteps removes step with id from list of accepted steps.
// After a step is accepted and has run to its final state it must be released before it can be run again.
func (ex *Executor) ReleaseSteps() error {
	ex.Lock()
	defer ex.Unlock()

	for k, v := range ex.running {
		ss := v.step.GetState()
		if step.IsStateFinal(ss) && v.exited {
			delete(ex.running, k)
		}
	}
	return nil
}

// Steps returns the Steps know to the executor.
func (ex *Executor) Steps() []step.Step {
	ex.Lock()
	defer ex.Unlock()

	var r []step.Step
	for _, rn := range ex.running {
		r = append(r, rn.step)
	}

	return r
}

// EnvironAdd adds env to the environment variables passed to the process under execution.
func (ex *Executor) EnvironAdd(env map[string]string) {
	if ex.Environ == nil {
		ex.Environ = make(map[string]string)
	}

	for k, v := range env {
		ex.Environ[k] = v
	}
}

// EnvironSlice returns the receivers environ as a slice of k=v strings.
func (ex *Executor) environSlice() []string {
	return util.KVSliceFromMap(ex.Environ)
}
