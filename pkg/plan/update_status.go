package plan

import (
	"fmt"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/step"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// UpdateStatusStep saves the step state in the environment status field.
func (p *Planner) UpdateStatusStep(status *v1.EnvironmentStatus, stp step.Step) error {
	m := stp.Meta()

	s := v1.StepStatus{
		State:              m.State,
		Message:            m.Msg,
		LastTransitionTime: metav1.Time{Time: m.LastUpdate},
	}

	if m.State == v1.StateReady {
		// step is completed successfully
		s.Hash = m.Hash
	}

	if status.Steps == nil {
		status.Steps = make(map[string]v1.StepStatus)
	}
	status.Steps[m.ID.ShortName()] = s

	return nil
}

// UpdateStatusConditions updates Status.Conditions to reflects the current state of the world.
// Ready = True when all steps are in ready state, Ready = False when some are not ready.
func (p *Planner) UpdateStatusConditions(nsn types.NamespacedName, status *v1.EnvironmentStatus) error {
	//steps := []step.ID{
	//	{Type: step.TypeInit},
	//	{Type: step.TypePlan},
	//	{Type: step.TypeApply},
	//}

	plan, ok := p.currentPlan(nsn)
	if !ok {
		return fmt.Errorf("expected plan for: %v", nsn)
	}

	var runningCnt, readyCnt, errorCnt, stateCnt, totalCnt int
	var latestTime metav1.Time
	for _, st := range plan { //steps /*TODO get allSteps(cspec) from Planner? needs nsn to get the right steps*/
		totalCnt++
		if s, ok := status.Steps[st.Meta().ID.ShortName()]; ok {
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
