package cmd

import (
	"context"
	"flag"
	"fmt"
	v1 "github.com/mmlt/environment-operator/api/clusterops/v1"
	xclientset "github.com/mmlt/environment-operator/pkg/generated/clientset/versioned"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
	"strings"
)

// NewCmdReset returns a command to reset an environment error.
func NewCmdReset() *cobra.Command {
	// flags
	var (
		stepName string
	)
	kubeConfigFlags := genericclioptions.NewConfigFlags(true)

	cmd := cobra.Command{
		Use:   "reset [--namespace name][--step name] environment-name",
		Short: "Reset an environment step so it will be re-executed",
		Long: `Reset an environment step so it will be re-executed. 
If no step name is provided the all steps in in error state will be reset.
NB only steps in Ready state can be reset.`,
		Args: cobra.ExactArgs(1),
		Run: func(c *cobra.Command, args []string) {
			cfg, err := kubeConfigFlags.ToRESTConfig()
			exitOnError(err)

			xClient, err := xclientset.NewForConfig(cfg)
			exitOnError(err)

			name := args[0]
			namespace := "default"
			if *kubeConfigFlags.Namespace != "" {
				namespace = *kubeConfigFlags.Namespace
			}

			environment, err := get(context.Background(), xClient, namespace, name)
			exitOnError(err)

			names, err := resetStep(environment, stepName)
			exitOnError(err)

			_, err = updateStatus(context.Background(), xClient, environment)
			exitOnError(err)

			fmt.Println("reset step(s):", strings.Join(names, " "))
			return
		},
	}

	// Add klog flags to cobra command.
	fs := flag.NewFlagSet("", flag.PanicOnError)
	klog.InitFlags(fs)
	cmd.Flags().AddGoFlagSet(fs)

	cmd.Flags().StringVar(&stepName, "step", "", "The name of the step to reset. Leave empty to reset steps in error state.")

	kubeConfigFlags.AddFlags(cmd.Flags())

	return &cmd
}

// Get gets an environment.
func get(ctx context.Context, client xclientset.Interface, namespace, name string) (*v1.Environment, error) {
	return client.
		ClusteropsV1().
		Environments(namespace).
		Get(ctx, name, metav1.GetOptions{})
}

// ResetStep modifies environment.status.steps by removing stepName or when stepName is empty by removing all steps
// in state error.
func resetStep(environment *v1.Environment, stepName string) ([]string, error) {
	var names []string

	if len(stepName) == 0 {
		// remove steps in state error
		for k, v := range environment.Status.Steps {
			if v.State == v1.StateError {
				delete(environment.Status.Steps, k)
				names = append(names, k)
			}
		}
		return names, nil
	}

	step, ok := environment.Status.Steps[stepName]
	if !ok {
		return nil, fmt.Errorf("no step with name: %s", stepName)
	}

	if step.State == v1.StateRunning {
		return nil, fmt.Errorf("can not reset step that is in state: %v", step.State)
	}

	delete(environment.Status.Steps, stepName)
	names = append(names, stepName)

	return names, nil
}

// UpdateStatus updates the status subresource of environment.
func updateStatus(ctx context.Context, client xclientset.Interface, environment *v1.Environment) (*v1.Environment, error) {
	return client.
		ClusteropsV1().
		Environments(environment.Namespace).
		UpdateStatus(ctx, environment, metav1.UpdateOptions{})
}
