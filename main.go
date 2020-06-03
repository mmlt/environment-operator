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

package main

import (
	"flag"
	"github.com/mmlt/environment-operator/pkg/client/addon"
	"github.com/mmlt/environment-operator/pkg/client/terraform"
	"github.com/mmlt/environment-operator/pkg/executor"
	"github.com/mmlt/environment-operator/pkg/plan"
	"github.com/mmlt/environment-operator/pkg/source"
	"k8s.io/klog"
	"k8s.io/klog/klogr"
	"os"
	"path/filepath"
	"time"

	clusteropsv1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/controllers"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = clusteropsv1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	metricsAddr := flag.String("metrics-addr", ":8080",
		"The address the metric endpoint binds to.")
	enableLeaderElection := flag.Bool("enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	selector := flag.String("selector", "clusterops.mmlt.nl/env=eu41tp",
		"Select the CR's that are handled by this operator instance.")
	syncPeriodInMin := flag.Int("sync-period-in-min", 10,
		"The max. interval time to check external sources like git.")

	// klog
	klog.InitFlags(nil)
	flag.Parse()

	log := klogr.New()
	ctrl.SetLogger(log)
	//TODO remove
	//ctrl.SetLogger(zap.New(func(o *zap.Options) {
	//	o.Development = true
	//}))

	// Setup manager.

	p := time.Duration(*syncPeriodInMin) * time.Minute
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: *metricsAddr,
		LeaderElection:     *enableLeaderElection,
		Port:               9443,
		SyncPeriod:         &p,
		//TODO Add RateLimiter that starts at 1m to max 10m see https://github.com/kubernetes-sigs/controller-runtime/issues/631
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Create environment reconciler and all it's dependencies.
	r := &controllers.EnvironmentReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("envop"),
		Log:      ctrl.Log.WithName("recon"),
		Selector: *selector,
	}
	r.Sources = &source.Sources{
		RootPath: filepath.Join(os.TempDir(), "envop"),
		Log:      r.Log.WithName("source"),
	}
	ao := &addon.Addon{
		Log: r.Log.WithName("addon"),
	}
	r.Planner = &plan.Planner{
		Log:   r.Log.WithName("plan"),
		Addon: ao,
	}
	tf := &terraform.Terraform{
		Log: r.Log.WithName("tf"),
	}
	r.Executor = &executor.Executor{
		UpdateSink: r,
		EventSink:  r,
		Terraform:  tf,
		Log:        r.Log.WithName("executor"),
	}

	err = r.SetupWithManager(mgr)
	if err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Environment")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	err = mgr.Start(ctrl.SetupSignalHandler())
	if err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
