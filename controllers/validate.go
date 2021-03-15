package controllers

import (
	"fmt"
	v1 "github.com/mmlt/environment-operator/api/v1"
)

// ValidateSpec returns an error when spec values are missing or wrong.
func validateSpec(es *v1.EnvironmentSpec) error {
	if len(es.Infra.AZ.Subscription) == 0 {
		return fmt.Errorf("spec.infra.az.subscription: at least 1 subscription expected")
	}
	//TODO Add validation logAnalyticsWorkspace.subscriptionName must be in spec.infra.subscription[]
	return nil
}

// ValidateClusterSpec returns an error when cluster values are missing or wrong.
func validateClusterSpec(cs *v1.ClusterSpec) error {
	//validations go here...
	_ = cs
	return nil
}
