package cmd

import (
	"context"
	"fmt"
	v1 "github.com/mmlt/environment-operator/api/clusterops/v1"
	"os"
)

const (
	CLIName        = "envopctl"
	ControllerName = "envop"
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
