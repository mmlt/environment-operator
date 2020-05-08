// Package Plan analyses the Environment and decides what Step should be executed next.
package plan

import (
	"github.com/go-logr/logr"
	"github.com/mmlt/environment-operator/pkg/addon"
)

// Plan decides what next step needs to be taken to move the infra to desired state.
// It can be viewed as a mediator between the Environment custom resource (containing current and desired state) and
// the steps executor.
// Plan NextStep() maps Environment.Spec and .Status to a Step to be executed next.
// Plan Update() puts the result of a Step in the Environment.Status fields.
type Plan struct {
	Addon addon.Addonr

	Log logr.Logger
}
