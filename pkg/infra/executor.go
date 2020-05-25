package infra

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/mmlt/environment-operator/pkg/step"
	"github.com/mmlt/environment-operator/pkg/terraform"
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	"sync"
)

var (
	MetricSteps = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "steps_total",
			Help: "Number of steps executed",
		},
	)
	MetricStepFailures = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "step_failures_total",
			Help: "Number of failed steps",
		},
	)
)

func init() {
	// Register custom metrics with the global prometheus registry.
	metrics.Registry.MustRegister(MetricSteps, MetricStepFailures)
}

// Executor executes Plans to create/update infrastructure.
// TODO update comment;
//Can execute multiple Plan's in parallel.
//Know how to serialize certain steps.
//Only contains (ephemeral) state that is common to all Plans.
//State that needs to be persisted is sent to the Reconciler.
//Some of this state might be on a local (persistent) volume like GIT, TFState.
//Can restart from state (in Plan)

// Executor runs Steps.
type Executor struct {
	// UpdateSink is notified when a plan has changed.
	UpdateSink step.Updater
	// EventSink is notified of Info and Warning events.
	EventSink step.Infoer
	// TF is the terraform implementation to use.
	Terraform terraform.Terraformer

	// Running is the map of running steps.
	running map[step.StepID]run

	Log logr.Logger
	sync.Mutex
}

//TODO do we still needs these for testing? if yes move to _test
/*
// Updater is a third party that wants to know about changes while executing a Step. //TODO remove
type Updater interface {
	Update(Step)
}

// Update is an adaptor from Update method to UpdaterFunc.
func (f UpdaterFunc) Update(step Step) {
	f(step)
}

// UpdaterFunc is a function that conforms to the Updater interface.
type UpdaterFunc func(Step)

// Infoer is a third party that wants to receive info/warning events. //TODO remove
// The main purpose is to help the user understand/debug the system.
type Infoer interface {
	Info(id StepID, msg string) error
	Warning(id StepID, msg string) error
}*/

// Run is the concurrent execution of a step.
type run struct {
	ctx    context.Context
	stopCh chan<- interface{}
	step   step.Step
}

// Accept attempts to execute another step and returns true if step is accepted.
// When a step is not accepted it should be retried later on.
// Progress is communicated over the receivers UpdateSink and EventSink.
func (ex *Executor) Accept(st step.Step) (bool, error) { //TODO rename step package to steps so we can use step as parameter
	if st == nil {
		// nothing to do
		return true, nil
	}

	ex.Lock()
	defer ex.Unlock()

	if _, ok := ex.running[st.Meta().ID]; ok {
		// step already running
		return true, nil
	}

	if len(ex.running) > 5 {
		// no worker available (max number reached)
		return false, nil
	}

	if ex.running == nil {
		ex.running = make(map[step.StepID]run)
	}

	// Execute step.
	r := run{
		ctx:    context.Background(),
		stopCh: make(chan<- interface{}),
		step:   st,
	}
	ex.running[st.Meta().ID] = r
	MetricSteps.Inc()
	go func() {
		//TODO behavior is Step dependent, receiver contains the work to do, parameters carry plumbing
		// Move sinks, Terraform, Log to StepMeta? StepMeta will be created by Planner (nice: Planner can decide on Terraform impl)
		ok := st.Execute(r.ctx, ex.EventSink, ex.UpdateSink, ex.Terraform, ex.Log)
		if !ok {
			MetricStepFailures.Inc()
		}

		ex.Lock()
		delete(ex.running, st.Meta().ID)
		ex.Unlock()

		close(r.stopCh)
	}()

	return true, nil
}

// Accept attempts to execute another step and returns true if step is accepted.
// When a step is not accepted it should be retried later on.
// Progress is communicated over the Updater and Infoer interfaces as passed to New().
/*func (ex *Executor) Accept(step Step) (bool, error) {
	if step == nil {
		// nothing to do
		return true, nil
	}

	ex.Lock()
	defer ex.Unlock()

	if _, ok := ex.running[step.Meta().ID]; ok {
		// step already running
		return true, nil
	}

	if len(ex.running) > 5 {
		// no worker available (max number reached)
		return false, nil
	}

	if ex.running == nil {
		ex.running = make(map[StepID]run)
	}

	// Execute step.
	r := run{
		ctx:    context.Background(),
		stopCh: make(chan<- interface{}),
		step:   step,
	}
	ex.running[step.Meta().ID] = r
	MetricSteps.Inc()
	go func() {
		//TODO behavior is Step dependent, receiver contains the work to do, parameters carry plumbing
		// Move sinks, Terraform, Log to StepMeta? StepMeta will be created by Planner (nice: Planner can decide on Terraform impl)
		ok := step.execute(r.ctx, ex.EventSink, ex.UpdateSink, ex.Terraform, ex.Log)
		if !ok {
			MetricStepFailures.Inc()
		}

		ex.Lock()
		delete(ex.running, step.Meta().ID)
		ex.Unlock()

		close(r.stopCh)
	}()

	return true, nil
}
*/
