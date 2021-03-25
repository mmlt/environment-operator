package main

import (
	"flag"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/mmlt/environment-operator/pkg/client/addon"
	"github.com/mmlt/environment-operator/pkg/client/azure"
	"github.com/mmlt/environment-operator/pkg/client/kubectl"
	"github.com/mmlt/environment-operator/pkg/client/terraform"
	"github.com/mmlt/environment-operator/pkg/cloud"
	"github.com/mmlt/environment-operator/pkg/plan"
	"github.com/mmlt/environment-operator/pkg/source"
	"github.com/mmlt/environment-operator/pkg/step"
	"github.com/mmlt/environment-operator/pkg/util"
	"k8s.io/klog"
	"k8s.io/klog/klogr"
	"os"
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
	requiredFlags := []string{"credentials-file", "vault", "workdir"}

	credentialsFile := flag.String("credentials-file", "",
		"file with JSON fields client_id, client_secret and tenant of a ServicePrincipal that is allowed to access the MasterKeyVault and AzureRM.")
	vault := flag.String("vault", "",
		"name of the KeyVault that contains secrets referenced from environment yaml.")
	workDir := flag.String("workdir", "/var/tmp/envop",
		"working directory")

	selector := flag.String("selector", "",
		"select which environment resources are handled by this operator instance.\n"+
			"when selector is not empty and the resource has a label 'clusterops.mmlt.nl/operator' that matches this flag the resource is handled.\n"+
			"when selector is empty all resources are handled.")
	syncPeriodInMin := flag.Int("sync-period-in-min", 10,
		"the max. interval time to check external sources like git.")
	allowedSteps := flag.String("allowed-steps", "",
		"a comma separated list of steps that are allowed to executed, empty allows all steps\n"+
			fmt.Sprintf("valid values: %v", step.Types))

	enableLeaderElection := flag.Bool("enable-leader-election", false,
		"enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	metricsAddr := flag.String("metrics-addr", ":8080",
		"address the metric endpoint binds to.")

	// klog
	klog.InitFlags(nil)
	log := klogr.New()
	ctrl.SetLogger(log)

	flag.Parse()
	if !flagsSet(log, requiredFlags...) {
		os.Exit(1)
	}

	steps, err := step.TypesFromString(*allowedSteps)
	if err != nil {
		log.Error(err, "flag --allowed-steps")
		os.Exit(1)
	}

	// Setup manager.

	p := time.Duration(*syncPeriodInMin) * time.Minute
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: *metricsAddr,
		LeaderElection:     *enableLeaderElection,
		Port:               9443,
		SyncPeriod:         &p,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	l := ctrl.Log.WithName("recon")

	// Create environment reconciler and all it's dependencies.
	r := &controllers.EnvironmentReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("envop"),
		Selector: *selector,
		Environ:  util.KVSliceToMap(os.Environ()),
	}
	r.Sources = &source.Sources{
		RootPath: *workDir,
		Log:      l,
	}
	r.Planner = &plan.Planner{
		AllowedStepTypes: steps,
		Log:              l,
		Cloud: &cloud.Azure{
			CredentialsFile: *credentialsFile,
			Vault:           *vault,
			Client: &azure.AZ{
				Log: l,
			},
			Log: l,
		},
		Terraform: &terraform.Terraform{},
		Kubectl: &kubectl.Kubectl{
			Log: l,
		},
		Azure: &azure.AZ{
			Log: l,
		},
		Addon: &addon.Addon{},
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

// FlagsSet returns true when all required flags are set.
// It logs flags with values and flags that are missing.
func flagsSet(log logr.Logger, flags ...string) bool {
	set := make(map[string]bool, len(flags))
	for _, f := range flags {
		set[f] = false
	}
	var ss []interface{}
	flag.Visit(func(f *flag.Flag) {
		set[f.Name] = true
		ss = append(ss, f.Name, f.Value.String())
	})
	log.Info("flags", ss...)

	var missing int
	for f, ok := range set {
		if !ok {
			log.Info("flag missing", "name", f)
			missing++
		}
	}

	return missing == 0
}
