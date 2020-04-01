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
	"github.com/mmlt/environment-operator/pkg/plan"
	"github.com/mmlt/environment-operator/pkg/source"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/mmlt/environment-operator/api/v1"
)

// EnvironmentReconciler reconciles a Environment object.
type EnvironmentReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	// Selector is a label=value string that selects the CR's that are handled by this instance.
	Selector string

	// Infrastructure is able to execute a plan to create/update infrastructure.
	Infrastructure Executer
}

type Executer interface {
	Execute(*plan.Plan)
}

// +kubebuilder:rbac:groups=clusterops.mmlt.nl,resources=environments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=clusterops.mmlt.nl,resources=environments/status,verbs=get;update;patch

func (r *EnvironmentReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("environment", req.NamespacedName.String())
	log.V(1).Info("Reconcile start")
	defer log.V(1).Info("Reconcile end")

	// TODO Client side filtering by r.Selector label until https://github.com/kubernetes-sigs/controller-runtime/issues/244 becomes available.

	// TODO add Policy checks

	// Get Environment Custom Resource.
	cr := &v1.Environment{}
	if err := r.Get(ctx, req.NamespacedName, cr); err != nil {
		log.V(1).Info("Unable to get Environment CR", "err", err)
		return ctrl.Result{}, ignoreNotFound(err)
	}

	// Create a plan and add source content.
	plan := plan.New(cr)
	for _, spec := range plan.Sources() {
		c := _source.New(spec)
		plan.AddContent(c)
	}

	//
	r.Infrastructure.Execute(plan)

	//r.Addon.Excute(plan)

	//r.Test.Run(plan)

	return ctrl.Result{}, nil
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
