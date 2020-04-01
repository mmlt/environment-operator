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
	"k8s.io/klog"
	"k8s.io/klog/klogr"
	"os"

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

	// klog
	klog.InitFlags(nil)
	flag.Set("v", "5")
	flag.Set("alsologtostderr", "true")
	flag.Parse()

	log := klogr.New()
	ctrl.SetLogger(log)
	//TODO remove
	//ctrl.SetLogger(zap.New(func(o *zap.Options) {
	//	o.Development = true
	//}))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: *metricsAddr,
		LeaderElection:     *enableLeaderElection,
		Port:               9443,
		//TODO Add RateLimiter that starts at 1m to max 10m see https://github.com/kubernetes-sigs/controller-runtime/issues/631
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	r := &controllers.EnvironmentReconciler{
		Client:   mgr.GetClient(),
		Log:      ctrl.Log.WithName("controllers").WithName("Environment"),
		Scheme:   mgr.GetScheme(),
		Selector: *selector,
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
