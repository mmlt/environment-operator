// Package executor performs steps to create environments.
//
// Executor accepts steps, runs them and keeps track of them.
package executor

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/mmlt/environment-operator/pkg/step"
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

// Executor performs Steps.
type Executor struct {
	// UpdateSink is notified when a plan has changed.
	UpdateSink step.Updater
	// EventSink is notified of Info and Warning events.
	EventSink step.Infoer
	//TODO remove
	//// Terraform is the terraform implementation to use.
	//Terraform terraform.Terraformer
	//// Azure is the azure cli implementation to use.
	//Azure azure.AZer

	// Running is the map of running steps.
	running map[step.ID]run

	Log logr.Logger
	sync.Mutex
}

// Run is the concurrent execution of a step.
type run struct {
	ctx    context.Context
	stopCh chan<- interface{}
	step   step.Step
}

// Accept attempts to execute another step and returns true if step is accepted.
// When a step is not accepted it should be retried later on.
// Progress is communicated over the receivers UpdateSink and EventSink.
func (ex *Executor) Accept(stp step.Step) (bool, error) {
	if stp == nil {
		// nothing to do
		return true, nil
	}

	ex.Lock()
	defer ex.Unlock()

	if _, ok := ex.running[stp.Meta().ID]; ok {
		// step already running
		return true, nil
	}

	if len(ex.running) > 5 {
		// no worker available (max number reached)
		return false, nil
	}

	if ex.running == nil {
		ex.running = make(map[step.ID]run)
	}

	// Execute step.
	r := run{
		ctx:    context.Background(),
		stopCh: make(chan<- interface{}),
		step:   stp,
	}
	ex.running[stp.Meta().ID] = r
	MetricSteps.Inc()
	go func() {
		log := ex.Log.WithName(stp.Meta().ID.ShortName())
		ok := stp.Execute(r.ctx, ex.EventSink, ex.UpdateSink, log)
		if !ok {
			MetricStepFailures.Inc()
		}

		ex.Lock()
		delete(ex.running, stp.Meta().ID)
		ex.Unlock()

		close(r.stopCh)
	}()

	return true, nil
}
