package kubectl

import (
	"github.com/ghodss/yaml"
	"github.com/go-logr/logr"
	"github.com/mmlt/environment-operator/pkg/util/exe"
	storagev1 "k8s.io/api/storage/v1"
)

// Kubectrler is able to perform kubectl cli commands.
type Kubectrler interface {
	// StorageClasses returns all the StorageClasses in the cluster addressed by kubeconfigPath.
	StorageClasses(kubeconfigPath string) ([]storagev1.StorageClass, error)
}

// Kubectl is able to perform kubectl cli commands.
type Kubectl struct {
	Log logr.Logger
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
