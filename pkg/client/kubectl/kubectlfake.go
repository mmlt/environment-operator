package kubectl

import (
	"fmt"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KubectlFake provides a Kubectler for testing.
type KubectlFake struct {
	// probePodState is the state of an fake 'probe' pod.
	// It mimics a Pod that is run like this:
	// 	kubectl -n kube-system
	//		run --generator=run-pod/v1 --restart OnFailure --image docker.io/curlimages/curl:7.72.0 probe
	//		-- sh -c 'until curl -ksS --max-time 2 https://kubernetes.default | grep Status ; do echo -n "Retry "; date -Iseconds; sleep 5 ; done'
	probePodState FakePodState
}

var _ Kubectrler = &KubectlFake{}

//go:generate stringer -type=FakePodState

type FakePodState int

const (
	FakePodUnknown FakePodState = iota
	FakePodRunning
	FakePodCompleted
	FakePodError
)

func (k *KubectlFake) PodState(kubeconfigPath, namespace, name string) (string, error) {
	switch k.probePodState {
	case FakePodRunning:
		k.probePodState = FakePodCompleted // simulate completed after one call.
		return "PodRunning", nil
	case FakePodCompleted:
		return "PodCompleted", nil
	case FakePodError:
		return "PodError", nil
	default:
		return "", nil
	}
}

func (k *KubectlFake) PodRun(kubeconfigPath, namespace, name, image, cmd string) error {
	switch k.probePodState {
	case FakePodRunning, FakePodCompleted, FakePodError:
		return fmt.Errorf("pod already present")
	default:
		k.probePodState = FakePodRunning
		return nil
	}
}

func (k *KubectlFake) PodLog(kubeconfigPath, namespace, name string) (string, error) {
	switch k.probePodState {
	case FakePodRunning:
		return "", nil
	case FakePodCompleted:
		r := `curl: (28) Resolving timed out after 2000 milliseconds
Retry 2020-09-08T07:01:06+0000
  "kind": "Status",
`
		return r, nil
	case FakePodError:
		return "", fmt.Errorf("an error happened")
	default:
		return "", fmt.Errorf("no pod present")
	}
}

func (k *KubectlFake) PodDelete(kubeconfigPath, namespace, name string) error {
	switch k.probePodState {
	case FakePodRunning, FakePodCompleted, FakePodError:
		k.probePodState = FakePodUnknown
		return nil
	default:
		return fmt.Errorf("no pod present")
	}
}

// StorageClasses returns all the StorageClasses in the cluster addressed by kubeconfigPath.
func (k KubectlFake) StorageClasses(kubeconfigPath string) ([]storagev1.StorageClass, error) {
	r := []storagev1.StorageClass{
		storagev1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					// AKS uses .beta.
					"storageclass.beta.kubernetes.io/is-default-class": "true",
				},
			},
		},
	}
	return r, nil
}

// WipeCluster removes resources before cluster delete.
func (k *KubectlFake) WipeCluster(kubeconfigPath string) error {
	return nil
}
