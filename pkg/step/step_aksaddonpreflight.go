package step

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/client/kubectl"
	"github.com/mmlt/environment-operator/pkg/util/backoff"
	"time"
)

// AKSAddonPreflightStep waits until all AKS specific preflight checks have been met.
type AKSAddonPreflightStep struct {
	Metaa

	/* Parameters */
	// KCPath is the path of the kube config file.
	KCPath string

	// Kubectl is the kubectl implementation to use.
	Kubectl kubectl.Kubectrler
}

// Meta returns a reference to the Metaa data of this Step.
func (st *AKSAddonPreflightStep) Meta() *Metaa {
	return &st.Metaa
}

// Execute node pool upgrade for a cluster.
func (st *AKSAddonPreflightStep) Execute(ctx context.Context, env []string, isink Infoer, usink Updater, log logr.Logger) bool {
	const (
		namespace = "kube-system"
		name      = "preflight"
	)
	var err error

	log.Info("start")

	st.State = v1.StateRunning
	usink.Update(st)

	// Remove possible leftover probe pod.
	s, _ := st.Kubectl.PodState(st.KCPath, namespace, name)
	if s != "" {
		// Pod already present, delete it
		err = st.Kubectl.PodDelete(st.KCPath, namespace, name)
		if err != nil {
			st.State = v1.StateError
			st.Msg = fmt.Sprintf("delete pod: %v", err)
			usink.Update(st)
			return false
		}
	}

	// Run probe.
	// TODO parameterize image (consider using envop config for this)
	err = st.Kubectl.PodRun(st.KCPath, namespace, name, "docker.io/curlimages/curl:7.72.0",
		"until curl -ksS --max-time 2 https://kubernetes.default | grep Status ; do date -Iseconds; sleep 5 ; done")
	if err != nil {
		st.State = v1.StateError
		st.Msg = fmt.Sprintf("run pod: %v", err)
		usink.Update(st)
		return false
	}
	// Check for completion.
	s = ""
	end := time.Now().Add(time.Minute)
	for exp := backoff.NewExponential(10 * time.Second); !time.Now().After(end); exp.Sleep() {
		s, err = st.Kubectl.PodState(st.KCPath, namespace, name)
		// err is included in Msg below
		if s == "PodCompleted" {
			break
		}
	}

	if s != "PodCompleted" {
		// Return with state Running so this step gets picked up again.
		var errMsg string
		if err != nil {
			errMsg = fmt.Sprintf(": %s", err.Error())
		} else {
			errMsg = fmt.Sprintf(": %s", s)
		}
		st.Msg = fmt.Sprintf("waiting for %s pod completion%s", name, errMsg)
		usink.Update(st)
		return true
	}

	st.State = v1.StateReady
	st.Msg = fmt.Sprintf("%s completed", name)
	usink.Update(st)

	return st.State == v1.StateReady
}
