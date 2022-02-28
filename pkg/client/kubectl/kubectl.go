package kubectl

import (
	"github.com/ghodss/yaml"
	"github.com/go-logr/logr"
	"github.com/mmlt/environment-operator/pkg/util/exe"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"strings"
)

// Kubectrler is able to perform kubectl cli commands.
type Kubectrler interface {
	// PodState returns the state of a Pod in the cluster addressed by kubeconfigPath.
	// State is FakePodRunning, FakePodCompleted, FakePodError or empty when no Pod is found.
	PodState(kubeconfigPath, namespace, name string) (string, error)
	// PodRun run a Pod
	PodRun(kubeconfigPath, namespace, name, image, cmd string) error
	// PodLog returns the log of a Pod.
	PodLog(kubeconfigPath, namespace, name string) (string, error)
	// PodDelete deletes a Pod.
	PodDelete(kubeconfigPath, namespace, name string) error
	// StorageClasses returns all the StorageClasses in the cluster addressed by kubeconfigPath.
	StorageClasses(kubeconfigPath string) ([]storagev1.StorageClass, error)
	// WipeCluster removes resources so all cluster nodes can be drained without errors.
	WipeCluster(kubeconfigPath string) error
}

// Kubectl is able to perform kubectl cli commands.
type Kubectl struct {
	Log logr.Logger
}

var _ Kubectrler = &Kubectl{}

// PodState returns the "Ready" status reason of the Pod in the cluster addressed by kubeconfigPath.
// State is PodRunning, PodCompleted, ContainersNotReady or empty when no Pod is found.
func (k Kubectl) PodState(kubeconfigPath, namespace, name string) (string, error) {
	args := []string{"--kubeconfig", kubeconfigPath, "-n", namespace, "get", "pod", name, "-o", "yaml"}
	o, _, err := exe.Run(k.Log, nil, "", "kubectl", args...)
	if err != nil && !strings.Contains(err.Error(), "Error from server (NotFound):") {
		// It's not an exit status 1 - Error from server (NotFound): pods "preflight" not found
		return "", err
	}

	var r v1.Pod
	err = yaml.Unmarshal([]byte(o), &r)
	if err != nil {
		return "", err
	}

	for _, v := range r.Status.Conditions {
		if v.Type == "Ready" {
			if v.Status == "True" {
				return "PodRunning", nil
			}
			return v.Reason, nil
		}
	}

	return "", nil
}

// PodRun runs a Pod.
func (k Kubectl) PodRun(kubeconfigPath, namespace, name, image, cmd string) error {
	args := []string{"--kubeconfig", kubeconfigPath, "-n", namespace,
		"run", "--restart", "OnFailure", "--image", image, name, "--", "sh", "-c", cmd}
	_, _, err := exe.Run(k.Log, nil, "", "kubectl", args...)

	return err
}

// PodLog returns the log of a Pod.
func (k Kubectl) PodLog(kubeconfigPath, namespace, name string) (string, error) {
	args := []string{"--kubeconfig", kubeconfigPath, "-n", namespace, "logs", name}
	o, _, err := exe.Run(k.Log, nil, "", "kubectl", args...)

	return o, err
}

// PodDelete deletes a Pod.
func (k Kubectl) PodDelete(kubeconfigPath, namespace, name string) error {
	args := []string{"--kubeconfig", kubeconfigPath, "-n", namespace, "delete", "pod", name}
	_, _, err := exe.Run(k.Log, nil, "", "kubectl", args...)

	return err
}

// StorageClasses returns all the StorageClasses in the cluster addressed by kubeconfigPath.
func (k Kubectl) StorageClasses(kubeconfigPath string) ([]storagev1.StorageClass, error) {
	args := []string{"--kubeconfig", kubeconfigPath, "get", "sc", "-o", "yaml"}
	o, _, err := exe.Run(k.Log, nil, "", "kubectl", args...)
	if err != nil {
		return nil, err
	}

	var r storagev1.StorageClassList
	err = yaml.Unmarshal([]byte(o), &r)
	if err != nil {
		return nil, err
	}

	return r.Items, nil
}

// Namespaces returns all the Namespaces matching labelSelector in the cluster addressed by kubeconfigPath.
func (k Kubectl) Namespaces(kubeconfigPath, labelSelector string) ([]v1.Namespace, error) {
	args := []string{"--kubeconfig", kubeconfigPath, "get", "ns", "-l", labelSelector, "-o", "yaml"}
	o, _, err := exe.Run(k.Log, nil, "", "kubectl", args...)
	if err != nil {
		return nil, err
	}

	var r v1.NamespaceList
	err = yaml.Unmarshal([]byte(o), &r)
	if err != nil {
		return nil, err
	}

	return r.Items, nil
}

// WipeCluster removes resources so all cluster nodes can be drained without errors.
// Arg kubeconfigRaw contains the kubeconfig file contents.
func (k Kubectl) WipeCluster(kubeconfigPath string) error {
	var err error
	var args []string

	args = []string{"--kubeconfig", kubeconfigPath, "delete", "validatingwebhookconfiguration", "--all", "--ignore-not-found=true"}
	_, _, err = exe.Run(k.Log, nil, "", "kubectl", args...)
	if err != nil {
		return err
	}

	args = []string{"--kubeconfig", kubeconfigPath, "delete", "mutatingwebhookconfiguration", "--all", "--ignore-not-found=true"}
	_, _, err = exe.Run(k.Log, nil, "", "kubectl", args...)
	if err != nil {
		return err
	}

	// delete namespace except those containing control-plane components
	nss, err := k.Namespaces(kubeconfigPath, "control-plane!=true")
	if err != nil {
		return err
	}

	for _, ns := range nss {
		if ns.Name == "default" || ns.Name == "kube-public" {
			continue
		}

		args = []string{"--kubeconfig", kubeconfigPath, "delete", "namespace", ns.Name}
		_, _, err = exe.Run(k.Log, nil, "", "kubectl", args...)
		if err != nil {
			return err
		}
	}

	return nil
}
