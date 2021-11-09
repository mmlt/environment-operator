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
	"github.com/mmlt/environment-operator/pkg/util"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog"
	"k8s.io/klog/klogr"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"time"
)

// NewCmdController returns a command to run the envop controller.
func NewCmdController() *cobra.Command {
	// flags
	var (
		credentialsFile      string
		vault                string
		workDir              string
		selector             string
		syncPeriodInMin      int
		allowedSteps         string
		enableLeaderElection bool
		metricsAddr          string
	)

	command := cobra.Command{
		Use:   "controller",
		Short: "Run the envop controller",
		Example: `
`,
		RunE: func(c *cobra.Command, args []string) error {
			log := klogr.New()
			ctrl.SetLogger(log)

			labelSet, err := labels.ConvertSelectorToLabelsMap(selector)
			if err != nil {
				return fmt.Errorf("flag --selector: %w", err)
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

			// Create environment reconciler and all it's dependencies.
			r := &controllers.EnvironmentReconciler{
				Client:   mgr.GetClient(),
				Scheme:   mgr.GetScheme(),
				Recorder: mgr.GetEventRecorderFor("envop"),
				Selector: selector,
				Environ:  util.KVSliceToMap(os.Environ()),
			}
			r.Sources = &source.Sources{
				RootPath: workDir,
				Log:      l,
			}
			r.Planner = &plan.Planner{
				AllowedStepTypes: steps,
				Log:              l,
				Cloud: &cloud.Azure{
					CredentialsFile: credentialsFile,
					Vault:           vault,
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
				Client: cluster.Client{
					Client: r.Client,
					Labels: labelSet,
				},
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

	command.Flags().StringVar(&credentialsFile, "credentials-file", "",
		"file with JSON fields client_id, client_secret and tenant of a ServicePrincipal that is allowed to access the MasterKeyVault and AzureRM.")
	must(command.MarkFlagRequired("credentials-file"))
	command.Flags().StringVar(&vault, "vault", "",
		"name of the KeyVault that contains secrets referenced from environment yaml.")
	must(command.MarkFlagRequired("vault"))
	command.Flags().StringVar(&workDir, "workdir", "/var/tmp/envop",
		"working directory")
	must(command.MarkFlagRequired("workdir"))

	command.Flags().StringVar(&selector, "selector", "",
		"select which environment resources are handled by this operator instance.\n"+
			"when selector is not empty and the resource has a label 'clusterops.mmlt.nl/operator' that matches this flag the resource is handled.\n"+
			"when selector is empty all resources are handled.")
	command.Flags().IntVar(&syncPeriodInMin, "sync-period-in-min", 10,
		"the max. interval time to check external sources like git.")
	command.Flags().StringVar(&allowedSteps, "allowed-steps", "",
		"a comma separated list of steps that are allowed to executed, empty allows all steps\n"+
			fmt.Sprintf("valid values: %v", step.Types))

	command.Flags().BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	command.Flags().StringVar(&metricsAddr, "metrics-addr", ":8080",
		"address the metric endpoint binds to.")

	return &command
}
