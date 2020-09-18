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
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sync"
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

	updateMutex sync.Mutex

	updateTally int // For debugging
}

const label = "clusterops.mmlt.nl/operator"

// +kubebuilder:rbac:groups=clusterops.mmlt.nl,resources=environments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=clusterops.mmlt.nl,resources=environments/status,verbs=get;update;patch

// Reconcile takes a k8s resource and attempts to adjust the underlying Environment to meet it's spec.
// The status of the k8s resource is updated to match the observed state of the Enviroment.
func (r *EnvironmentReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("request", req.NamespacedName)
	log.V(1).Info("Start Reconcile")

	// TODO add Policy checks
	// TODO enable/disable operator via annotations?

	// Get Environment.
	cr := &v1.Environment{}
	if err := r.Get(ctx, req.NamespacedName, cr); err != nil {
		log.V(1).Info("Unable to get kind Environment", "err", err)
		return ctrl.Result{}, ignoreNotFound(err)
	}

	// Ignore environments that do not match selector.
	// For now client side filtering, for server side see https://github.com/kubernetes-sigs/controller-runtime/issues/244
	if len(r.Selector) > 0 {
		v, ok := cr.Labels[label]
		if !ok || v != r.Selector {
			log.V(2).Info("label selector doesn't match", "label", label, "value", v, "selector", r.Selector)
			return ctrl.Result{}, nil
		}
	}

	// Get ClusterSpecs with defaults.
	cspec, err := flattenedClusterSpec(cr.Spec)
	if err != nil {
		// Spec contains error (needs user to fix it first so do noy retry).  TODO warn user via webhook
		return ctrl.Result{}, fmt.Errorf("Spec: %w", err)
	}

	// Register sources.
	err = r.Sources.Register(req.NamespacedName, "", cr.Spec.Infra.Source)
	if err != nil {
		return ctrl.Result{Requeue: true}, fmt.Errorf("register infra source: %w", err)
	}
	for _, sp := range cspec {
		err := r.Sources.Register(req.NamespacedName, sp.Name, sp.Addons.Source)
		if err != nil {
			return ctrl.Result{Requeue: true}, fmt.Errorf("register cluster source: %w", err)
		}
	}

	// Ask Planner for next step.
	step, err := r.Planner.NextStep(req.NamespacedName, r.Sources, cr.Spec.Destroy, cr.Spec.Infra, cspec, cr.Status)
	if err != nil {
		return ctrl.Result{Requeue: true}, fmt.Errorf("plan next step: %w", err)
	}

	// Try to run step.
	accepted, err := r.Executor.Accept(step)
	if err != nil {
		return ctrl.Result{Requeue: true}, fmt.Errorf("accept step for execution: %w", err)
	}
	if !accepted {
		return ctrl.Result{Requeue: true}, nil
	}

	// While steps are running reconcile often.
	if hasStepState(cr.Status.Steps, v1.StateRunning) {
		return ctrl.Result{RequeueAfter: time.Minute}, nil
		//return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

func hasStepState(stps map[string]v1.StepStatus, state v1.StepState) bool {
	for _, stp := range stps {
		if stp.State == state {
			return true
		}
	}
	return false
}

// Update Environment status with step.
func (r *EnvironmentReconciler) Update(step step.Step) {
	// Implementation:
	// Update serializes writes to environment status but does not rate limit them.

	log := r.Log.WithName("Update").V(2)

	step.Meta().LastUpdate = time.Now()

	nsn := types.NamespacedName{
		Namespace: step.Meta().ID.Namespace,
		Name:      step.Meta().ID.Name,
	}

	// Serialize status updates.
	r.updateMutex.Lock()
	defer r.updateMutex.Unlock()

	r.updateTally++
	if step.Meta().State == v1.StateError {
		log.Info(string(step.Meta().State), "stepName", step.Meta().ID.ShortName(), "update", r.updateTally, "step", step)
	} else {
		log.Info(string(step.Meta().State), "stepName", step.Meta().ID.ShortName(), "update", r.updateTally)
	}

	for i := 0; i < 10; i++ {
		// Get Environment.
		ctx := context.Background()
		cr := &v1.Environment{}
		err := r.Get(ctx, nsn, cr)
		if err != nil {
			r.Log.Error(err, "get kind Environment")
			return
		}

		// Merge step into CR.
		err = r.Planner.UpdateStatusStep(&cr.Status, step)
		if err != nil {
			r.Log.Error(err, "update status state")
			return
		}

		err = r.Planner.UpdateStatusConditions(nsn, &cr.Status)
		if err != nil {
			r.Log.Error(err, "update status condition")
			return
		}

		// Write back to server.
		ctx = context.Background()
		err = r.Status().Update(ctx, cr)
		if err == nil {
			return
		}

		if apierrors.IsConflict(err) {
			// the object has been modified (code 409)
			//apierrors.SuggestsClientDelay()				err.ErrStatus.Code == apierrors.IsConflict()
			time.Sleep(time.Second)
			log.Info("update status conflict", "retry", i, "update", r.updateTally)

			continue
		}
		r.Log.Error(err, "update status of kind Environment")

		return
	}
	log.Info("update status of kind Environment give up", "update", r.updateTally)
	return
}

func (r *EnvironmentReconciler) Info(id step.ID, msg string) error {
	return r.event(id, "Normal", msg)
}

func (r *EnvironmentReconciler) Warning(id step.ID, msg string) error {
	return r.event(id, "Warning", msg)
}

func (r *EnvironmentReconciler) event(id step.ID, eventtype, msg string) error {
	//r.Log.V(2).Info("Event", "type", "Normal", "id", id, "msg", msg)
	//TODO use gvk := obj.GetObjectKind().GroupVersionKind() to replace hardcoded values?
	// With UID the events show with the Object.
	// Pass UID around? OR pass Object around (instead of nsn)?
	o := &corev1.ObjectReference{
		Kind:      "Environment",
		Namespace: id.Namespace,
		Name:      id.Name,
		//UID:             r.x,
		APIVersion: "clusterops.mmlt.nl/v1",
	}
	r.Recorder.Event(o, eventtype, id.ShortName(), msg)
	return nil
}

// SetupWithManager initializes the receiver and adds it to mgr.
func (r *EnvironmentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Environment{}).
		Complete(r)
}

// IgnoreNotFound makes NotFound errors disappear.
// We generally want to ignore (not requeue) NotFound errors, since we'll get a
// reconciliation request once the object exists, and requeuing in the meantime
// won't help.
func ignoreNotFound(err error) error {
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

// FlattenedClusterSpec returns []ClusterSpec merged with default values.
func flattenedClusterSpec(in v1.EnvironmentSpec) ([]v1.ClusterSpec, error) {
	var r []v1.ClusterSpec
	for _, c := range in.Clusters {
		cs := in.Defaults.DeepCopy()
		mergo.Merge(cs, c, mergo.WithOverride)
		//TODO assert that required values are set and valid.
		r = append(r, *cs)
	}

	return r, nil
}
