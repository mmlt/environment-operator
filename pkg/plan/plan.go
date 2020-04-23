// Package Plan analyses the Environment and decides what Step should be executed next.
package plan

import (
	"github.com/go-logr/logr"
	v1 "github.com/mmlt/environment-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
	"sync"
)

// Plan decides what next step needs to be taken to move the infra to desired state.
// It can be viewed as a mediator between the Environment custom resource (containing current and desired state) and
// the steps executor.
// Plan NextStep() maps Environment.Spec and .Status to a Step to be executed next.
// Plan Update() puts the result of a Step in the Environment.Status fields.
type Plan struct {
	Log logr.Logger
}

// Conditions provides methods to query and update EnvironmentConditions.
type conditions struct {
	//TODO remove; map only helps in one case when type names are known but most cases use name matching.
	//inner map[string]v1.EnvironmentCondition
	inner        []v1.EnvironmentCondition
	sync.RWMutex //TODO is write locking used at all?
}

/*func newConditions(cs []v1.EnvironmentCondition) *conditions {
	r := &conditions{
		inner: map[string]v1.EnvironmentCondition{},
	}

	for _, c := range cs {
		r.inner[c.Type] = c
	}

	return r
}*/

// Collect returns the conditions that match the parameters.
func (cs *conditions) collect(typePrefix string, status metav1.ConditionStatus, reason v1.EnvironmentConditionReason) []v1.EnvironmentCondition {
	var r []v1.EnvironmentCondition
	cs.RLock()
	defer cs.RUnlock()
	for _, c := range cs.inner {
		if c.Reason == reason && c.Status == status && strings.HasPrefix(c.Type, typePrefix) {
			r = append(r, c)
		}
	}
	return r
}

// Matches returns the number of type matches and the number type, status and reason matches.
func (cs *conditions) matches(typePrefix string, status metav1.ConditionStatus, reason v1.EnvironmentConditionReason) (t, tsr int) {
	cs.RLock()
	defer cs.RUnlock()
	for _, c := range cs.inner {
		if strings.HasPrefix(c.Type, typePrefix) {
			t++
			if c.Status == status && c.Reason == reason {
				tsr++
			}
		}
	}
	return
}

// Unknown returns true if there are no conditions with typePrefix or the conditions have status ConditionUnknown.
func (cs *conditions) unknown(typePrefix string) bool {
	cs.RLock()
	defer cs.RUnlock()
	for _, c := range cs.inner {
		if strings.HasPrefix(c.Type, typePrefix) {
			if c.Status != metav1.ConditionUnknown {
				return false
			}
		}
	}
	return true
}

// Any returns true if one or more conditions match.
func (cs *conditions) any(typePrefix string, status metav1.ConditionStatus, reason v1.EnvironmentConditionReason) bool {
	_, tsr := cs.matches(typePrefix, status, reason)
	return tsr > 0
}

// After answers true if the left side type LastTransitionTime is more recent than the next LastTransitionTime
// (type[n]time > type[n+1]time)
// Missing types are ignored.
func (cs *conditions) after(types ...string) bool {
	cs.RLock()
	defer cs.RUnlock()

	var t *metav1.Time
	for _, ty := range types {
		/*TODO remove
		if c, ok := cs.inner[ty]; ok {
			if t == nil {
				t = &c.LastTransitionTime
				continue
			}

			if !t.After(c.LastTransitionTime.Time) {
				return false
			}

			t = &c.LastTransitionTime
		}*/
		for _, c := range cs.inner {
			if c.Type == ty {
				x := c.LastTransitionTime

				if t == nil {
					t = &x
					continue
				}

				if !t.After(c.LastTransitionTime.Time) {
					return false
				}

				t = &x
			}
		}
	}
	return true
}
