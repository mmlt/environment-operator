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
	"github.com/mmlt/environment-operator/pkg/executor"
	"github.com/mmlt/environment-operator/pkg/plan"
	"github.com/mmlt/environment-operator/pkg/source"
	"github.com/mmlt/environment-operator/pkg/step"
	"github.com/robfig/cron/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
	Log      logr.Logger

	// Selector much match the value of resource label to be handled this instance.
	// An empty Selector matches all resources.
	Selector string

	// Sources fetches tf or yaml source code.
	Sources *source.Sources

	// Planner decides on the next step to execute based on Environment.
	Planner *plan.Planner

	// Executor executes Steps.
	Executor *executor.Executor

	// Invocation counters
	reconTally int
}

const label = "clusterops.mmlt.nl/operator"

// RequeueIntervalWhenRunnning is the time between reconciliations when a step is running.
const requeueIntervalWhenRunnning = 10 * time.Second

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

	log := r.Log.WithValues("request", req.NamespacedName)

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

	// Update environment status with the current state-of-the-world.
	err = syncStatusWithExecutor(&cr.Status, r.Executor)
	if err != nil {
		return requeueSoon, fmt.Errorf("sync status with executor: %w", err)
	}

	// Plan work.
	var stp step.Step
	if !hasStepState(cr.Status.Steps, v1.StateError) {
		running := stepFilter(cr.Status, v1.StateRunning)
		if len(running) == 0 {
			// No step is running.
			stp, err = r.planNextStepAndUpdateStatus(cr, req, log)
			if err != nil {
				return requeueSoon, err
			}
		} else {
			// A step is already running.
			stp = r.Planner.ExistingStep(req.NamespacedName, running[0])
		}
	}

	// Save state-of-the-world.
	updateStatusConditions(&cr.Status)
	err = r.saveStatus(cr)
	if err != nil {
		return requeueNow, fmt.Errorf("save status: %w", err)
	}

	// Perform work.
	r.Executor.ReleaseSteps()
	_, err = r.Executor.Accept(stp)
	if err != nil {
		return requeueNow, fmt.Errorf("accept step for execution: %w", err)
	}

	if stp != nil {
		return requeueSoon, nil
	}

	return noRequeue, nil
}

// PlanNextStepAndUpdateStatus makes a plan and select the next step.
func (r *EnvironmentReconciler) planNextStepAndUpdateStatus(cr *v1.Environment, req ctrl.Request, log logr.Logger) (step.Step, error) {
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
	// and select the next step.
	stp, err := syncStatusWithPlan(&cr.Status, pln)
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

// SyncStatusWithExecutor updates status.steps with the status of executing steps.
// When status has recorded a step in final state it tells the executor to release that step.
func syncStatusWithExecutor(status *v1.EnvironmentStatus, ex *executor.Executor) error {
	if status.Steps == nil {
		status.Steps = make(map[string]v1.StepStatus)
	}

	for _, stp := range ex.Steps() {
		/*TODO		n := stp.GetID().ShortName()

		if ss, ok := status.Steps[n]; ok && step.IsStateFinal(ss.State) {
			err := ex.Release(stp.GetID())
			if err != nil {
				return err
			}
		}*/

		ss := v1.StepStatus{
			State:              stp.GetState(),
			Message:            stp.GetMsg(),
			LastTransitionTime: metav1.Time{Time: stp.GetLastUpdate()},
		}

		if ss.LastTransitionTime.IsZero() {
			// might happen when executor has Accepted step but the step hasn't started executing yet.
			ss.LastTransitionTime = metav1.Time{Time: timeNow()}
		}

		if stp.GetState() == v1.StateReady {
			// step is completed successfully
			ss.Hash = stp.GetHash()
		}

		status.Steps[stp.GetID().ShortName()] = ss
	}

	return nil
}

// syncStatusWithPlan update status.steps with plan and returns the next step to execute.
func syncStatusWithPlan(status *v1.EnvironmentStatus, plan []step.Step) (step.Step, error) {
	if status.Steps == nil {
		status.Steps = make(map[string]v1.StepStatus)
	}

	var r step.Step
	for _, stp := range plan {
		id := stp.GetID()

		// Get status step state.
		stStp, ok := status.Steps[id.ShortName()]
		if !ok {
			// first time this step is seen.
			stStp = v1.StepStatus{
				Message:            "new",
				LastTransitionTime: metav1.Time{Time: timeNow()},
			}
			status.Steps[id.ShortName()] = stStp
		}

		if stStp.Hash == stp.GetHash() {
			// step is at desired state.
			continue
		}

		if r == nil {
			// first step in plan with non-matching hash.
			r = stp
		}

		if stStp.State == v1.StateReady {
			// clear state of a step that needs to be run again.
			stStp.State = ""

			// Consider also doing to reverse: set stStepSate = v1.StateReady when hashes match.
			// For example in the following sequence of events a step will run again even it's not strictly necessary;
			//	1. step source or parameter are changed
			//	2. stStp.State is cleared because the hashes don't match anymore
			//	3. changes from 1 are undone
		}
	}

	// TODO remove stStp that are not in plan anymore.
	// Use case: envop has updated status.steps of a cr and then envop is reconfigured with --allowed-steps allowing
	// less steps. This results in updateStatusConditions to take take steps that are not relevant anymore into account
	// when setting Condition 'Ready'

	return r, nil
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

	return
}

// SaveStatus writes the status to the API server.
func (r *EnvironmentReconciler) saveStatus(cr *v1.Environment) error {
	log := r.Log.WithName("saveStatus")

	nsn := types.NamespacedName{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}

	for i := 0; i < 10; i++ {
		ctx := context.Background()
		c := &v1.Environment{}
		err := r.Get(ctx, nsn, c)
		if err != nil {
			log.Error(err, "get kind", "retry", i)
			time.Sleep(time.Second) //TODO expo backoff
			continue
		}

		c.Status = cr.Status

		ctx = context.Background()
		err = r.Status().Update(ctx, c)
		if err != nil {
			log.Error(err, "update kind status", "retry", i)
			time.Sleep(time.Second) //TODO expo backoff
			continue
		}

		return nil
	}

	return fmt.Errorf("too many errors, give up")
}

func (r *EnvironmentReconciler) Update(meta step.Meta) {
	log := r.Log.WithName("Update")

	nsn := types.NamespacedName{
		Namespace: meta.GetID().Namespace,
		Name:      meta.GetID().Name,
	}
	// Get Environment.
	ctx := context.Background()
	cr := &v1.Environment{}
	err := r.Get(ctx, nsn, cr)
	if err != nil {
		log.Error(err, "get kind Environment")
	}

	r.Recorder.Event(cr, "Normal", meta.GetID().ShortName()+string(meta.GetState()), meta.GetMsg())
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

// ValidateSpec returns an error when spec values are missing or wrong.
func validateSpec(es *v1.EnvironmentSpec) error {
	if len(es.Infra.AZ.Subscription) == 0 {
		return fmt.Errorf("spec.infra.az.subscription: at least 1 subscription expected")
	}
	//TODO Add validation logAnalyticsWorkspace.subscriptionName must be in spec.infra.subscription[]
	return nil
}

// ValidateClusterSpec returns an error when cluster values are missing or wrong.
func validateClusterSpec(cs *v1.ClusterSpec) error {
	//validations go here...
	_ = cs
	return nil
}

// StepFilter returns the names of the steps that match state.
func stepFilter(status v1.EnvironmentStatus, state v1.StepState) []string {
	var r []string
	for n, s := range status.Steps {
		if s.State == state {
			r = append(r, n)
		}
	}
	return r
}

func hasStepState(stps map[string]v1.StepStatus, state v1.StepState) bool {
	for _, stp := range stps {
		if stp.State == state {
			return true
		}
	}
	return false
}
