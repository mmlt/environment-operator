package cmd

import (
	"context"
	"fmt"
	v1 "github.com/mmlt/environment-operator/api/clusterops/v1"
	"os"
)

const (
	// CLIName is the name of the command line interface.
	CLIName = "envopctl"
	// ControllerName is the name used by this operator when interacting with Kubernetes.
	ControllerName = "envop"
	// LabelKey is the label that selects the resources accessed by this operator.
	LabelKey = "clusterops.mmlt.nl/operator"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func exitOnError(err error) {
	if err != nil {
		if err != context.Canceled {
			fmt.Fprintf(os.Stderr, "An error occurred: %v\n", err)
		}
		os.Exit(1)
	}
}

// StatusCondition returns the named condition from environment.status.conditions.
func statusCondition(environment *v1.Environment, condition string) (*v1.EnvironmentCondition, bool) {
	if environment == nil {
		return nil, false
	}

	for _, c := range environment.Status.Conditions {
		if c.Type == condition {
			return &c, true
		}
	}

	return nil, false
}
