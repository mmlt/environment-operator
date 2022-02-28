package cmd

import (
	"flag"
	"fmt"
	clusteropsv1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/controllers"
	"github.com/mmlt/environment-operator/pkg/client/addon"
	"github.com/mmlt/environment-operator/pkg/client/azure"
	"github.com/mmlt/environment-operator/pkg/client/kubectl"
	"github.com/mmlt/environment-operator/pkg/client/terraform"
	"github.com/mmlt/environment-operator/pkg/cloud"
	"github.com/mmlt/environment-operator/pkg/cluster"
	"github.com/mmlt/environment-operator/pkg/plan"
	"github.com/mmlt/environment-operator/pkg/source"
	"github.com/mmlt/environment-operator/pkg/step"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog"
	"k8s.io/klog/klogr"
	"os"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"time"
)

// NewDryrunControllerCmd returns a command to run the envop controller in dryrun mode.
func NewDryrunControllerCmd() *cobra.Command {
	// flags
	var (
		selector             string
		allowedSteps         string
		syncPeriodInMin      int
		enableLeaderElection bool
		metricsAddr          string
	)

	command := cobra.Command{
		Use:   "dryruncontroller",
		Short: "Run an envop controller in dryrun mode",
		Long: `In dryrun mode the controller doesn't write to external systems.
Because the dryruncontroller works with fake data the processed environment(yaml) must:
- have a budget of add=1, delete=1, update=2 or more
- have a single cluster called "mycluster"
`,
		Example: `
`,
		RunE: func(c *cobra.Command, args []string) error {
			log := klogr.New()
			ctrl.SetLogger(log)

			labelSet := labels.Set{}
			if selector != "" {
				labelSet[LabelKey] = selector
			}

			steps, err := step.TypesFromString(allowedSteps)
			if err != nil {
				return fmt.Errorf("flag --allowed-steps: %w", err)
			}

			// Setup manager.
			scheme := runtime.NewScheme()
			_ = clientgoscheme.AddToScheme(scheme)
			_ = clusteropsv1.AddToScheme(scheme)

			p := time.Duration(syncPeriodInMin) * time.Minute
			mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
				Scheme:             scheme,
				MetricsBindAddress: metricsAddr,
				LeaderElection:     enableLeaderElection,
				Port:               9443,
				SyncPeriod:         &p,
			})
			if err != nil {
				return fmt.Errorf("unable to start manager: %w", err)
			}

			l := ctrl.Log.WithName("recon")

			cl := &cloud.Fake{}
			r := &controllers.EnvironmentReconciler{
				Client:   mgr.GetClient(),
				Scheme:   mgr.GetScheme(),
				Recorder: mgr.GetEventRecorderFor("envop"),
				LabelSet: labelSet,
				Environ: map[string]string{
					"PATH": "/usr/local/bin", //kubectl-tmplt uses kubectl
				},
				Cloud: cl,
			}

			r.Sources = &source.Sources{
				RootPath: filepath.Join(os.TempDir(), "envop"),
				Log:      l,
			}

			az := &azure.AZFake{}
			az.SetupFakeResults()
			tf := &terraform.TerraformFake{
				Log: l,
			}
			tf.SetupFakeResultsForCreate(nil)
			kc := &kubectl.KubectlFake{}
			ao := &addon.AddonFake{}
			ao.SetupFakeResult()
			clc := cluster.Client{
				Client: r.Client,
				Labels: labelSet,
			}
			r.Planner = &plan.Planner{
				AllowedStepTypes: steps,
				Terraform:        tf,
				Kubectl:          kc,
				Azure:            az,
				Cloud:            cl,
				Addon:            ao,
				Client:           clc,
				Log:              l,
			}

			err = r.SetupWithManager(mgr)
			if err != nil {
				return fmt.Errorf("unable to create controller: %w", err)
			}

			err = mgr.Start(ctrl.SetupSignalHandler())
			if err != nil {
				return fmt.Errorf("problem running manager: %w", err)
			}

			return nil
		},
	}

	// Add klog flags to cobra command.
	fs := flag.NewFlagSet("", flag.PanicOnError)
	klog.InitFlags(fs)
	command.Flags().AddGoFlagSet(fs)

	command.Flags().StringVar(&selector, "selector", "",
		"select which environment resources are handled by this operator instance.\n"+
			"when selector is not empty and the resource has a label 'clusterops.mmlt.nl/operator' that matches this flag the resource is handled.\n"+
			"when selector is empty all resources are handled.")
	command.Flags().StringVar(&allowedSteps, "allowed-steps", "",
		"a comma separated list of steps that are allowed to executed, empty allows all steps\n"+
			fmt.Sprintf("valid values: %v", step.Types))

	command.Flags().IntVar(&syncPeriodInMin, "sync-period-in-min", 10,
		"the max. interval time to check external sources like git.")
	command.Flags().BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	command.Flags().StringVar(&metricsAddr, "metrics-addr", ":8080",
		"address the metric endpoint binds to.")

	return &command
}
