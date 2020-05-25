package plan

import (
	"fmt"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/step"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UpdateStatusConditions with step results.
func (p *Plan) UpdateStatusConditionOLD(status *v1.EnvironmentStatus, step step.Step) error {
	/*TODO implemente conditions
	m := step.Meta()

	c := v1.EnvironmentCondition{
		LastTransitionTime: metav1.Time{Time: m.LastUpdate},
		Message:            m.Msg,
	}
	//c.Type = m.ID.Type
	c.Type = m.ID.Type.String()
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
	}*/

	return nil
}

func (p *Plan) UpdateStatusStep(status *v1.EnvironmentStatus, st step.Step) error {

	m := st.Meta()

	s := v1.StepStatus{
		State:              m.State,
		Message:            m.Msg,
		Hash:               m.Hash,
		LastTransitionTime: metav1.Time{Time: m.LastUpdate},
	}

	if status.Steps == nil {
		status.Steps = make(map[string]v1.StepStatus)
	}
	status.Steps[m.ID.ShortName()] = s

	return nil
}

/*// UpdateStatusValues with step results. //TODO remove
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
*/

/*// UpdateStatusSynced summarizes conditions in status.Synced.
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
}*/

// UpdateStatusConditions updates conditions;
// Ready = True when all steps are in ready state, Ready = False when some are not ready.
func (p *Plan) UpdateStatusConditions(status *v1.EnvironmentStatus) error {
	steps := []step.StepID{
		{Type: step.StepTypeInit},
		{Type: step.StepTypePlan},
		{Type: step.StepTypeApply},
	}

	var runningCnt, readyCnt, errorCnt, stateCnt, totalCnt int
	var latestTime metav1.Time
	for _, id := range steps /*TODO get allSteps(cspec) from Plan? needs nsn to get the right steps*/ {
		totalCnt++
		if s, ok := status.Steps[id.ShortName()]; ok {
			stateCnt++
			switch s.State {
			case v1.StateRunning:
				runningCnt++
			case v1.StateReady:
				readyCnt++
			case v1.StateError:
				errorCnt++
			}

			if s.LastTransitionTime.After(latestTime.Time) {
				latestTime = s.LastTransitionTime
			}
		}
	}

	state := metav1.ConditionUnknown
	var reason v1.EnvironmentConditionReason
	if stateCnt > 0 {
		state = metav1.ConditionFalse
		reason = v1.ReasonRunning
		if readyCnt == totalCnt {
			state = metav1.ConditionTrue
			reason = v1.ReasonReady
		}
	}
	if errorCnt > 0 {
		reason = v1.ReasonFailed
	}

	c := v1.EnvironmentCondition{
		Type:               "Ready", //TODO define in API types
		Status:             state,
		Reason:             reason,
		Message:            fmt.Sprintf("%d of %d ready, %d running, %d error(s)", readyCnt, totalCnt, runningCnt, errorCnt),
		LastTransitionTime: latestTime,
	}

	var exists bool
	for i, v := range status.Conditions {
		if v.Type == c.Type {
			exists = true
			status.Conditions[i] = c
			break
		}
	}
	if !exists {
		status.Conditions = append(status.Conditions, c)
	}

	return nil
}
