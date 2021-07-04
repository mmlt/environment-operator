package cmd

import (
	"context"
	"flag"
	"fmt"
	v1 "github.com/mmlt/environment-operator/api/clusterops/v1"
	xclientset "github.com/mmlt/environment-operator/pkg/generated/clientset/versioned"
	xinformers "github.com/mmlt/environment-operator/pkg/generated/informers/externalversions"
	"github.com/spf13/cobra"
	"io"
	"io/ioutil"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	yaml2 "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"os"
	"time"
)

// NewCmdApply returns a command to apply an environment to an envop controller.
func NewCmdApply() *cobra.Command {
	// flags
	var (
		timeout  time.Duration
		filename string
	)
	kubeConfigFlags := genericclioptions.NewConfigFlags(true)

	cmd := cobra.Command{
		Use:   "apply -f file [--timeout duration]",
		Short: "Apply an environment to envop",
		Long: `Apply an environment to an envop controller and wait for status condition Ready.
Write events to stdout while waiting`,
		Run: func(c *cobra.Command, args []string) {
			cfg, err := kubeConfigFlags.ToRESTConfig()
			exitOnError(err)

			xClient, err := xclientset.NewForConfig(cfg)
			exitOnError(err)
			kubeClient, err := kubernetes.NewForConfig(cfg)
			exitOnError(err)

			// get resource to apply
			var b []byte
			if filename == "-" {
				b, err = io.ReadAll(os.Stdin)
			} else {
				b, err = ioutil.ReadFile(filename)
			}
			exitOnError(err)

			environment := &v1.Environment{}
			err = yaml2.Unmarshal(b, environment)
			exitOnError(err)

			if *kubeConfigFlags.Namespace != "" {
				environment.Namespace = *kubeConfigFlags.Namespace
			}
			if environment.Namespace == "" {
				environment.Namespace = "default"
			}

			// apply
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			t := time.Now()

			_, err = apply(ctx, xClient, environment)
			exitOnError(err)

			fmt.Printf("applied environment %s/%s\n", environment.Namespace, environment.Name)

			if timeout < time.Millisecond {
				// skip the waiting
				return
			}

			fmt.Printf("wait for ready status\n")

			logEvents(ctx, kubeClient, environment, t)

			err = waitReady(ctx, xClient, environment.Namespace, environment.Name, t)
			exitOnError(err)

			fmt.Printf("ready in %s\n", time.Now().Sub(t).Truncate(time.Second))
			return
		},
	}

	// Add klog flags to cobra command.
	fs := flag.NewFlagSet("", flag.PanicOnError)
	klog.InitFlags(fs)
	cmd.Flags().AddGoFlagSet(fs)

	cmd.Flags().DurationVar(&timeout, "timeout", time.Hour, "The length of time to wait for envop ready, zero means don't wait. Any other values should contain a corresponding time unit (e.g. 1s, 2m, 3h).")
	cmd.Flags().StringVarP(&filename, "filename", "f", "", "The environment to apply (- reads from stdin).")
	must(cmd.MarkFlagRequired("filename"))

	kubeConfigFlags.AddFlags(cmd.Flags())

	return &cmd
}

// Apply applies an environment.
func apply(ctx context.Context, client xclientset.Interface, environment *v1.Environment) (*v1.Environment, error) {
	if environment == nil {
		return nil, fmt.Errorf("environment provided to Apply must not be nil")
	}
	data, err := json.Marshal(environment)
	if err != nil {
		return nil, err
	}
	name := environment.Name
	if len(name) == 0 {
		return nil, fmt.Errorf("environment.Name must be provided to Apply")
	}
	return client.
		ClusteropsV1().
		Environments(environment.Namespace).
		Patch(ctx, name, types.ApplyPatchType, data, metav1.PatchOptions{FieldManager: CLIName})
}

// WaitReady waits for condition[Ready]==True and returns the Reason.
// Only environment objects newer than t are checked.
func waitReady(ctx context.Context, client xclientset.Interface, namespace, name string, t time.Time) error {
	ch := make(chan v1.EnvironmentCondition)

	xInformerFactory := xinformers.NewSharedInformerFactory(client, time.Minute)
	environmentInformer := xInformerFactory.Clusterops().V1().Environments().Informer()
	environmentInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			if x, ok := newObj.(*v1.Environment); ok {
				if x.Namespace == namespace && x.Name == name {
					if c, ok := statusCondition(x, "Ready"); ok {
						if c.LastTransitionTime.Time.After(t) {
							ch <- *c
						}
					}
				}
			}
		},
	})

	xInformerFactory.Start(ctx.Done())

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case c := <-ch:
			switch c.Reason {
			case v1.ReasonReady:
				return nil
			case v1.ReasonFailed:
				return fmt.Errorf("an envop step failed") //TODO we can give a better message; check Steps and print message
			case "", v1.ReasonRunning:
				// NOP
			default:
				return fmt.Errorf("unexpected Reason while waiting for condition Ready: %v", c.Reason)
			}
		}
	}
}

// LogEvents writes Environment related Events newer than t to stdout.
func logEvents(ctx context.Context, client kubernetes.Interface, environment *v1.Environment, t time.Time) {
	logFn := func(obj interface{}) {
		if o, ok := obj.(*eventsv1.Event); ok {
			if o.Regarding.Kind == environment.Kind &&
				o.Regarding.Name == environment.Name &&
				o.Regarding.Namespace == environment.Namespace &&
				o.CreationTimestamp.After(t) {
				fmt.Println("Event:", o.CreationTimestamp, "action", o.Action, "type", o.Type, "reason", o.Reason, "note", o.Note)
			}
		}
	}
	kubeInformerFactory := informers.NewSharedInformerFactory(client, 10*time.Minute)
	eventInformer := kubeInformerFactory.Events().V1().Events().Informer()
	eventInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    logFn,
		UpdateFunc: func(_, obj interface{}) { logFn(obj) },
	})
	kubeInformerFactory.Start(ctx.Done())
}
