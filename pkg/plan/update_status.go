package plan

import (
	"fmt"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/infra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

// UpdateStatusCondition with step results.
func (p *Plan) UpdateStatusCondition(status *v1.EnvironmentStatus, step infra.Step) error {
	m := step.Meta()

	c := v1.EnvironmentCondition{
		LastTransitionTime: metav1.Time{Time: m.LastUpdate},
		Message:            m.Msg,
	}
	c.Type = m.ID.Type
	if m.ID.ClusterName != "" {
		c.Type = c.Type + strings.Title(m.ID.ClusterName)
	}
	switch m.State {
	case infra.StepStateUnknown:
		c.Status = metav1.ConditionUnknown
	case infra.StepStateRunning:
		c.Status = metav1.ConditionTrue
		c.Reason = v1.ReasonRunning
	case infra.StepStateReady:
		c.Status = metav1.ConditionFalse
		c.Reason = v1.ReasonReady
	case infra.StepStateError:
		c.Status = metav1.ConditionFalse
		c.Reason = v1.ReasonFailed
	default:
		return fmt.Errorf("(fatal) unexpected step state: %T", m.State)
	}

	var exists bool
	for i, v := range status.Conditions {
		if v.Type == c.Type { //TODO should v.time > c.time?
			status.Conditions[i] = c
			exists = true
			break
		}
	}
	if !exists {
		status.Conditions = append(status.Conditions, c)
	}

	return nil
}

// UpdateStatusValues with step results.
func (p *Plan) UpdateStatusValues(status *v1.EnvironmentStatus, step infra.Step) error {
	switch x := step.(type) {
	case *infra.InitStep:
	case *infra.PlanStep:
		status.Infra.PAdded = x.Added
		status.Infra.PChanged = x.Changed
		status.Infra.PDeleted = x.Deleted
	case *infra.ApplyStep:
		status.Infra.Added = x.Added
		status.Infra.Changed = x.Changed
		status.Infra.Deleted = x.Deleted
		if x.State == infra.StepStateReady {
			// update hash meaning we've successfully applied the desired config and parameters.
			status.Infra.Hash = x.Hash
		}
	case *infra.KubeconfigStep:

	case *infra.AddonStep:

	default:
		return fmt.Errorf("(fatal) unexpected step type: %T", step)
	}

	return nil
}

// UpdateStatusSynced summarizes conditions in status.Synced.
// NB. SyncedReady doesn't mean we're done, we can also be in between steps.
func (p *Plan) UpdateStatusSynced(status *v1.EnvironmentStatus) error {
	var f, r bool
	for _, c := range status.Conditions {
		switch c.Reason {
		case v1.ReasonRunning:
			r = true
		case v1.ReasonFailed:
			f = true
		}
	}
	status.Synced = v1.SyncedReady
	if r {
		status.Synced = v1.SyncedSyncing
	}
	if f {
		status.Synced = v1.SyncedError
	}

	return nil
}
