/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/imdario/mergo"
	"github.com/mmlt/environment-operator/pkg/plan"
	"github.com/mmlt/environment-operator/pkg/source"
	"github.com/mmlt/environment-operator/pkg/step"
	"github.com/mmlt/environment-operator/pkg/util"
	"github.com/robfig/cron/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"

	v1 "github.com/mmlt/environment-operator/api/v1"
)

// EnvironmentReconciler reconciles a Environment object.
type EnvironmentReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	// Selector much match the value of resource label to be handled this instance.
	// An empty Selector matches all resources.
	Selector string

	// Sources fetches tf or yaml source code.
	Sources *source.Sources

	// Planner decides on the next step to execute based on Environment.
	Planner *plan.Planner

	// Environ are the environment variables presented to the steps.
	Environ map[string]string

	// Invocation counters
	reconTally int
}

const label = "clusterops.mmlt.nl/operator"

// TimeNow for testing.
var timeNow = time.Now

// +kubebuilder:rbac:groups=clusterops.mmlt.nl,resources=environments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=clusterops.mmlt.nl,resources=environments/status,verbs=get;update;patch

// Reconcile takes an Environment custom resource and attempts to converge the target environment to the desired state.
// The status of the k8s resource is updated to match the observed state of the Envirnoment.
func (r *EnvironmentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var (
		noRequeue   = ctrl.Result{}
		requeueNow  = ctrl.Result{Requeue: true}
		requeueSoon = ctrl.Result{RequeueAfter: 10 * time.Second}
	)

	log := logr.FromContext(ctx).WithName("Reconcile")
	ctx = logr.NewContext(ctx, log)

	r.reconTally++
	log.V(1).Info("Start Reconcile", "tally", r.reconTally)
	defer log.V(1).Info("End Reconcile", "tally", r.reconTally)

	// Get Environment Custom Resource (deep copy).
	cr := &v1.Environment{}
	if err := r.Get(ctx, req.NamespacedName, cr); err != nil {
		log.V(2).Info("unable to get kind Environment (retried)", "error", err)
		return requeueSoon, ignoreNotFound(err)
	}

	// Ignore environments that do not match selector.
	// (implemented as client side filtering, for server side see https://github.com/kubernetes-sigs/controller-runtime/issues/244)
	if len(r.Selector) > 0 {
		v, ok := cr.Labels[label]
		if !ok || v != r.Selector {
			log.V(2).Info("ignored, label selector doesn't match", "label", label, "value", v, "selector", r.Selector)
			return noRequeue, nil
		}
	}

	// Ignore when not within time schedule.
	ok, err := inSchedule(cr.Spec.Infra.Schedule, time.Now())
	if err != nil {
		// Schedule contains error (needs user to fix it first so do noy retry).
		r.Recorder.Event(cr, "Warning", "Config", "infra.schedule:"+err.Error())
		return noRequeue, fmt.Errorf("spec.infra.schedule: %w", err)
	}
	if !ok {
		log.V(2).Info("outside schedule", "schedule", cr.Spec.Infra.Schedule)
		return noRequeue, nil
	}

	if hasStepState(cr.Status.Steps, v1.StateError) {
		// Needs step state reset to continue.
		return noRequeue, nil
	}

	// Plan work.
	stp, err := r.nextStep(cr, req, log)

	// save planned steps (some steps might need to be re-executed)
	err = r.saveStatus2(ctx, cr)
	if err != nil {
		return requeueNow, fmt.Errorf("save status: %w", err)
	}

	// Execute work.
	if stp != nil {
		stp.SetOnUpdate(func(meta step.Meta) {
			log1 := logr.FromContext(ctx).WithName("OnUpdate")
			ctx1 := logr.NewContext(ctx, log)

			e := meta.GetLastError()
			m := meta.GetMsg()
			if e != nil {
				log.Error(e, m)
			}
			s := meta.GetState()
			log1.Info("callback", "msg", m, "state", s, "id", meta.GetID().ShortName())
			r.update(ctx1, cr, meta)
		})
		env := util.KVSliceFromMap(r.Environ)
		stp.Execute(ctx, env)
	}

	return noRequeue, nil
}

// NextStep fetches sources, makes a plan, updates cr and returns the next step.
// Step is nil if there is nothing to do.
func (r *EnvironmentReconciler) nextStep(cr *v1.Environment, req ctrl.Request, log logr.Logger) (step.Step, error) {
	// Get ClusterSpecs with defaults.
	cspec, err := flattenedClusterSpec(cr.Spec)
	if err != nil {
		// Spec contains error (needs user to fix it first so do noy retry).
		r.Recorder.Event(cr, "Warning", "Config", err.Error())
		return nil, fmt.Errorf("spec: %w", err)
	}

	// Register and fetch sources.
	err = r.Sources.Register(req.NamespacedName, "", cr.Spec.Infra.Source)
	if err != nil {
		return nil, fmt.Errorf("source: register infra: %w", err)
	}
	for _, sp := range cspec {
		err = r.Sources.Register(req.NamespacedName, sp.Name, sp.Addons.Source)
		if err != nil {
			return nil, fmt.Errorf("source: register cluster: %w", err)
		}
	}
	err = r.Sources.FetchAll()
	if err != nil {
		log.Error(err, "source: fetch")
	}
	// update workspaces
	_, err = r.Sources.Get(req.NamespacedName, "")
	if err != nil {
		return nil, fmt.Errorf("source: get infra: %w", err)
	}
	for _, sp := range cspec {
		_, err = r.Sources.Get(req.NamespacedName, sp.Name)
		if err != nil {
			return nil, fmt.Errorf("source: get cluster: %w", err)
		}
	}

	// Make a plan
	pln, err := r.Planner.Plan(req.NamespacedName, r.Sources, cr.Spec.Destroy, cr.Spec.Infra, cspec)
	if err != nil {
		return nil, fmt.Errorf("plan: %w", err)
	}
	stp, err := getStepAndSyncStatusWithPlan(&cr.Status, pln, log)
	if err != nil {
		return nil, fmt.Errorf("sync status with plan: %w", err)
	}
	return stp, nil
}

// InSchedule returns true when time now is in CRON schedule or the schedule is empty.
//
//  Field name   | Mandatory? | Allowed values  | Allowed special characters
//  ----------   | ---------- | --------------  | --------------------------
//  Minutes      | Yes        | 0-59            | * / , -
//  Hours        | Yes        | 0-23            | * / , -
//  Day of month | Yes        | 1-31            | * / , - ?
//  Month        | Yes        | 1-12 or JAN-DEC | * / , -
//  Day of week  | Yes        | 0-6 or SUN-SAT  | * / , - ?
//
// Special characters:
//   * always
//   / interval, for example */5 is every 5m
//   , list, for example MON,FRI in DayOfWeek field
//   - range, for example 20-04 in hour field
// See https://godoc.org/github.com/robfig/cron#Parser
func inSchedule(schedule string, now time.Time) (bool, error) {
	if schedule == "" {
		return true, nil
	}

	p := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	sc, err := p.Parse(schedule)
	if err != nil {
		return false, err
	}

	next := sc.Next(now)

	ok := next.Sub(now).Minutes() < 10.0

	return ok, nil
}

// getStepAndSyncStatusWithPlan update status.steps with plan and returns the next step to execute.
// Return nil if no step is to be executed.
func getStepAndSyncStatusWithPlan(status *v1.EnvironmentStatus, plan []step.Step, log logr.Logger) (step.Step, error) {
	if status.Steps == nil {
		status.Steps = make(map[string]v1.StepStatus)
	}

	var r step.Step
	for _, stp := range plan {
		shortName := stp.GetID().ShortName()

		// Get status step state.
		stStp, ok := status.Steps[shortName]
		if !ok {
			// first time this step is seen.
			stStp = v1.StepStatus{
				Message:            "new",
				LastTransitionTime: metav1.Time{Time: timeNow()},
			}
		}

		if stStp.Hash == stp.GetHash() {
			// step is at desired state.
			if stStp.State != v1.StateReady {
				// state is inconsistent, fix it
				log.Info("inconsistency in status: step state with matching hash should have State=Ready", "step", stStp)
				stStp.State = v1.StateReady
			}
			continue
		}

		if r == nil {
			// the first step in the plan with a non-matching hash.
			r = stp
		}

		if stStp.State == v1.StateReady {
			// clear state of a step that needs to be run again because its hash has changed.
			stStp.State = ""

			// Consider also doing to reverse: set stStepSate = v1.StateReady when hashes match.
			// For example in the following sequence of events a step will run again even it's not strictly necessary;
			//	1. step source or parameter are changed
			//	2. stStp.State is cleared because the hashes don't match anymore
			//	3. changes from 1 are undone
		}

		status.Steps[shortName] = stStp
	}

	// TODO remove stStp that are not in plan anymore.
	// Use case: envop has updated status.steps of a cr and then envop is reconfigured with --allowed-steps allowing
	// less steps. This results in updateStatusConditions to take take steps that are not relevant anymore into account
	// when setting Condition 'Ready'

	// status consistency checks
	if r == nil {
		// if no step is selected to be run all steps must be Ready
		for _, stStp := range status.Steps {
			if stStp.State != v1.StateReady {
				log.Info("inconsistency in status: if no step is to be run all states must be Ready", "step", stStp)
			}
		}
	}

	return r, nil
}

// SaveStatus writes the status to the API server.
func (r *EnvironmentReconciler) saveStatus2(ctx context.Context, cr *v1.Environment) error {
	updateStatusConditions(&cr.Status)

	log := logr.FromContext(ctx)
	log.Info("saveStatus", "status", cr.Status)

	return r.Status().Update(ctx, cr)
}

// UpdateStatusConditions updates Status.Conditions to reflect steps state.
// Ready = True when all steps are in their final state, Reason is Ready or Failed.
// Ready = False when a step is running, Reason is Running.
func updateStatusConditions(status *v1.EnvironmentStatus) {
	var runningCnt, readyCnt, errorCnt, totalCnt int
	var latestTime metav1.Time

	for _, st := range status.Steps {
		totalCnt++
		switch st.State {
		case v1.StateRunning:
			runningCnt++
		case v1.StateReady:
			readyCnt++
		case v1.StateError:
			errorCnt++
		}

		if st.LastTransitionTime.After(latestTime.Time) {
			latestTime = st.LastTransitionTime
		}
	}

	c := v1.EnvironmentCondition{
		Type: "Ready", //TODO define in API types
	}
	switch {
	case errorCnt > 0:
		c.Status = metav1.ConditionTrue
		c.Reason = v1.ReasonFailed
	case runningCnt > 0:
		c.Status = metav1.ConditionFalse
		c.Reason = v1.ReasonRunning
	case readyCnt == totalCnt:
		c.Status = metav1.ConditionTrue
		c.Reason = v1.ReasonReady
	default:
		c.Status = metav1.ConditionUnknown
		c.Reason = ""
	}
	c.Message = fmt.Sprintf("%d/%d ready, %d running, %d error(s)", readyCnt, totalCnt, runningCnt, errorCnt)
	c.LastTransitionTime = latestTime

	var exists bool
	for i, v := range status.Conditions {
		if v.Type == c.Type {
			exists = true
			status.Conditions[i] = c
			break
		}
	}
	if !exists {
		status.Conditions = append(status.Conditions, c)
	}
}

// Update updates cr.Status with meta, writes the status to the API Server and records an Event.
func (r *EnvironmentReconciler) update(ctx context.Context, cr *v1.Environment, meta step.Meta) {
	log := logr.FromContext(ctx)

	shortname := meta.GetID().ShortName()

	r.Recorder.Event(cr, "Normal", shortname+string(meta.GetState()), meta.GetMsg())

	// copy meta to step
	ss := cr.Status.Steps[shortname]
	ss.State = meta.GetState()
	ss.Message = meta.GetMsg()
	ss.LastTransitionTime = metav1.Time{Time: timeNow()}
	if ss.State == v1.StateReady {
		// step has completed.
		ss.Hash = meta.GetHash()
	}
	cr.Status.Steps[shortname] = ss

	err := r.saveStatus2(ctx, cr)
	if err != nil {
		// failing to save a final state will result in re-execution of the step
		log.Error(err, "saveStatus")
	}
}

// SetupWithManager initializes the receiver and adds it to mgr.
func (r *EnvironmentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Environment{}).
		Complete(r)
}

// IgnoreNotFound makes NotFound errors disappear.
// We generally want to ignore (not requeue) NotFound errors, since we'll get a
// reconciliation request once the object exists, and re-queuing in the meantime
// won't help.
func ignoreNotFound(err error) error {
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

// FlattenedClusterSpec returns []ClusterSpec merged with default values.
// Return an error on spec validation issues.
func flattenedClusterSpec(in v1.EnvironmentSpec) ([]v1.ClusterSpec, error) {
	err := validateSpec(&in)
	if err != nil {
		return nil, fmt.Errorf("validate spec: %w", err)
	}

	var r []v1.ClusterSpec
	for _, c := range in.Clusters {
		cs := in.Defaults.DeepCopy()

		err = mergo.Merge(cs, c, mergo.WithOverride)
		if err != nil {
			return nil, fmt.Errorf("merge spec.cluster %s: %w", c.Name, err)
		}

		err = validateClusterSpec(cs)
		if err != nil {
			return nil, fmt.Errorf("validate spec.cluster %s: %w", c.Name, err)
		}

		r = append(r, *cs)
	}

	return r, nil
}

// HasStepState returns true when one of the stps is in state.
func hasStepState(stps map[string]v1.StepStatus, state v1.StepState) bool {
	for _, stp := range stps {
		if stp.State == state {
			return true
		}
	}
	return false
}
