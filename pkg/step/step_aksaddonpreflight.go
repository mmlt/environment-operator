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

// Execute runs preflight checks.
func (st *AKSAddonPreflightStep) Execute(ctx context.Context, _ []string) {
	const (
		namespace = "kube-system"
		name      = "preflight"
	)
	var err error

	log := logr.FromContext(ctx).WithName("AKSAddonPreflightStep")
	log.Info("start")

	st.update(v1.StateRunning, "check api-server connection")

	// Remove possible leftover probe pod.
	s, _ := st.Kubectl.PodState(st.KCPath, namespace, name)
	if s != "" {
		// Pod already present, delete it
		err = st.Kubectl.PodDelete(st.KCPath, namespace, name)
		if err != nil {
			st.error2(err, "delete pod")
			return
		}
	}

	// Run probe.
	// TODO parameterize image (consider using envop config for this)
	err = st.Kubectl.PodRun(st.KCPath, namespace, name, "docker.io/curlimages/curl:7.80.0",
		"until curl -ksS --max-time 2 https://kubernetes.default | grep Status ; do date -Iseconds; sleep 5 ; done")
	if err != nil {
		st.error2(err, "run pod")
		return
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
		st.update(v1.StateRunning, fmt.Sprintf("waiting for %s pod completion%s", name, errMsg))
		return
	}

	// Wait for AKS to have resources deployed.
	// On 20200821 when AKS provisioning is completed (according to terraform) it still takes 5 minutes or more for
	// the default StorageClass to appear. During that time window PVC's that don't set 'storageClass:' will fail.
	st.update(v1.StateRunning, "waiting for default StorageClass")
	err = st.waitForDefaultStorageClass()
	if err != nil {
		st.error2(err, "waiting for default StorageClass")
		return
	}

	st.update(v1.StateReady, fmt.Sprintf("%s completed", name))
}

// WaitForDefaultStorageClass waits until the target cluster contains a StorageClass with 'default' annotation.
func (st *AKSAddonPreflightStep) waitForDefaultStorageClass() error {
	var errTally int

	end := time.Now().Add(10 * time.Minute)
	for exp := backoff.NewExponential(30 * time.Second); !time.Now().After(end); exp.Sleep() {
		scs, err := st.Kubectl.StorageClasses(st.KCPath)
		if err != nil {
			errTally++
			if errTally > 3 {
				return fmt.Errorf("kubectl: %w", err)
			}
			continue
		}
		for _, sc := range scs {
			v := sc.Annotations["storageclass.kubernetes.io/is-default-class"]
			if v == "" {
				// AKS uses .beta.
				v = sc.Annotations["storageclass.beta.kubernetes.io/is-default-class"]
			}
			if v == "true" {
				return nil
			}
		}
	}

	return fmt.Errorf("time-out")
}
