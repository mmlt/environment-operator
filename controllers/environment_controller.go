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
	"github.com/mmlt/environment-operator/pkg/infra"
	"github.com/mmlt/environment-operator/pkg/plan"
	"github.com/mmlt/environment-operator/pkg/source"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sync"
	"time"

	v1 "github.com/mmlt/environment-operator/api/v1"
)

// EnvironmentReconciler reconciles a Environment object.
type EnvironmentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger

	// TODO Selector is a label=value string that selects the CR's that are handled by this instance.
	Selector string

	// Sources fetches tf or yaml source code.
	Sources *source.Sources

	// Plan decides on the next step to execute based on Environment.
	Plan *plan.Plan

	// Executor executes Steps.
	Executor *infra.Executor

	updateMutex sync.Mutex

	updateTally int // For debugging
}

// +kubebuilder:rbac:groups=clusterops.mmlt.nl,resources=environments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=clusterops.mmlt.nl,resources=environments/status,verbs=get;update;patch

// Reconcile takes a k8s resource and attempts to adjust the underlying Environment to meet it's spec.
// The status of the k8s resource is updated to match the observed state of the Enviroment.
func (r *EnvironmentReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("request", req.NamespacedName)
	log.V(1).Info("Start Reconcile")

	// TODO Client side filtering by r.Selector label until https://github.com/kubernetes-sigs/controller-runtime/issues/244 becomes available.

	// TODO add Policy checks

	// Get Environment.
	cr := &v1.Environment{}
	if err := r.Get(ctx, req.NamespacedName, cr); err != nil {
		log.V(1).Info("Unable to get kind Environment", "err", err)
		return ctrl.Result{}, ignoreNotFound(err)
	}

	// Flatten and validate Environment.Spec
	spec, err := processSpec(cr.Spec)
	if err != nil {
		// Spec contains error (needs user to fix it first so do noy retry).  TODO warn user via webhook
		return ctrl.Result{}, fmt.Errorf("Spec: %w", err)
	}

	if len(spec) == 0 {
		log.V(1).Info("Nothing to do (no clusters in spec)")
		return ctrl.Result{}, nil
	}

	// Register sources.
	err = r.registerSources(req.NamespacedName, spec)
	if err != nil {
		return ctrl.Result{Requeue: true}, fmt.Errorf("register sources: %w", err)
	}

	// Plan next step.
	step, err := r.Plan.NextStep(req.NamespacedName, r.Sources, spec, cr.Status)
	//step, err := r.Plan.NextStep(r.Sources, cr)
	if err != nil {
		return ctrl.Result{Requeue: true}, fmt.Errorf("plan next step: %w", err)
	}

	log.V(2).Info("Next Step", "step", step)

	// Accept step for execution.
	accepted, err := r.Executor.Accept(step)
	if err != nil {
		return ctrl.Result{Requeue: true}, fmt.Errorf("accept step for execution: %w", err)
	}

	return ctrl.Result{Requeue: !accepted}, nil
}

// Update Environment status with step.
func (r *EnvironmentReconciler) Update(step infra.Step) {
	// Implementation:
	// Update serializes writes to environment status but does not rate limit them.

	log := r.Log.V(2)

	step.Meta().LastUpdate = time.Now()

	nsn := types.NamespacedName{
		Namespace: step.Meta().ID.Namespace,
		Name:      step.Meta().ID.Name,
	}

	// Serialize status updates.
	r.updateMutex.Lock()
	defer r.updateMutex.Unlock()

	r.updateTally++
	log.Info("Update", "step", step, "tally", r.updateTally)

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
		err = r.Plan.UpdateStatusCondition(&cr.Status, step)
		if err != nil {
			r.Log.Error(err, "update status condition")
			return
		}
		err = r.Plan.UpdateStatusValues(&cr.Status, step)
		if err != nil {
			r.Log.Error(err, "update status values")
			return
		}
		err = r.Plan.UpdateStatusSynced(&cr.Status)
		if err != nil {
			r.Log.Error(err, "update status synced")
			return
		}

		//TODO remove or change to V(3) because the conditions list can be long
		//log.Info("Update Status", "conditions", cr.Status.Conditions, "retry", i, "tally", r.updateTally)

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
			log.Info("update status conflict", "retry", i, "tally", r.updateTally)

			continue
		}
		r.Log.Error(err, "update status of kind Environment")

		return
	}
	log.Info("update status of kind Environment give up", "tally", r.updateTally)
	return
}

func (r *EnvironmentReconciler) Info(id infra.StepID, msg string) error {
	r.Log.V(2).Info("Info", "id", id, "msg", msg)
	//TODO implement EventRecorder Info
	return nil
}

func (r *EnvironmentReconciler) Warning(id infra.StepID, msg string) error {
	r.Log.V(2).Info("Warning", "id", id, "msg", msg)
	//TODO implement EventRecorder Warning
	return nil
}

// SetupWithManager initializes the receiver and adds it to mgr.
func (r *EnvironmentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Environment{}).
		Complete(r)
}

// RegisterSources updates to local copies of all sources in spec.
func (r *EnvironmentReconciler) registerSources(nsn types.NamespacedName, spec []v1.ClusterSpec) error {
	for i, sp := range spec {
		if i == 0 {
			// infra is fetched once as it is common to all clusters.
			err := r.Sources.Register(nsn, source.Ninfra, sp.Infrastructure.Source)
			if err != nil {
				return err
			}
		}
		err := r.Sources.Register(nsn, sp.Name, sp.Addons.Source)
		if err != nil {
			return err
		}

		//TODO r.ource.Register(sp.Test.Sources)
	}
	return nil
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

// ProcessSpec returns Environment.Spec in a flattened and validate form.
func processSpec(in v1.EnvironmentSpec) ([]v1.ClusterSpec, error) {
	var r []v1.ClusterSpec
	for _, c := range in.Clusters {
		//TODO check for values that should not be set a cluster level (should no override the default value)
		cs := in.Defaults.DeepCopy()
		mergo.Merge(cs, c, mergo.WithOverride)
		//TODO assert that required values are set and valid.
		r = append(r, *cs)
	}

	return r, nil
}
