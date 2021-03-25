package step

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/go-logr/logr"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/client/kubectl"
	"github.com/mmlt/environment-operator/pkg/client/terraform"
	"github.com/mmlt/environment-operator/pkg/cloud"
	"github.com/mmlt/environment-operator/pkg/util"
	"github.com/mmlt/environment-operator/pkg/util/backoff"
	"io/ioutil"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api/v1"
	"sigs.k8s.io/yaml"
	"strings"
	"time"
)

// KubeconfigStep reads data from terraform and creates a kubeconfig file.
type KubeconfigStep struct {
	Metaa

	/* Parameters */

	// TFPath is the path to the directory containing terraform code.
	TFPath string
	// ClusterName is the name of the k8s cluster to create a kube config for.
	ClusterName string
	// KCPath is the place were the kube config file is written.
	KCPath string
	// Access is the token access terraform state with.
	Access string

	// Cloud provides generic cloud functionality.
	Cloud cloud.Cloud
	// Terraform is the terraform implementation to use.
	Terraform terraform.Terraformer
	// Kubectl is the kubectl implementation to use.
	Kubectl kubectl.Kubectrler
}

// Run a step.
func (st *KubeconfigStep) Execute(ctx context.Context, env []string) {
	log := logr.FromContext(ctx).WithName("KubeconfigStep")
	ctx = logr.NewContext(ctx, log)
	log.Info("start")

	st.update(v1.StateRunning, "get kubeconfig")

	sp, err := st.Cloud.Login()
	if err != nil {
		st.error2(err, "login")
		return
	}
	xenv := terraformEnviron(sp, st.Access)
	env = util.KVSliceMergeMap(env, xenv)

	o, err := st.Terraform.Output(ctx, env, st.TFPath)
	if err != nil {
		st.error2(err, "terraform output")
		return
	}

	kc, err := kubeconfig(o, st.ClusterName)
	if err != nil {
		st.error2(err, "kubeconfig from terraform output")
		return
	}

	err = ioutil.WriteFile(st.KCPath, kc, 0600)
	if err != nil {
		st.error2(err, "write kubeconfig")
		return
	}

	//TODO move to AKSAddonPreflight
	// Wait for AKS to have resources deployed.
	// On 20200821 when AKS provisioning is completed (according to terraform) it still takes 5 minutes or more for
	// the default StorageClass to appear. During that time window PVC's that don't set 'storageClass:' will fail.
	st.update(v1.StateRunning, "check default StorageClass is present")
	err = st.waitForDefaultStorageClass()
	if err != nil {
		st.error2(err, "waiting for default StorageClass")
		return
	}

	st.update(v1.StateReady, "default StorageClass is present")
}

// WaitForDefaultStorageClass waits until the target cluster contains a StorageClass with 'default' annotation.
func (st *KubeconfigStep) waitForDefaultStorageClass() error {
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

// Kubeconfig returns a kube config from a terraform output value json.
func kubeconfig(json map[string]interface{}, clusterName string) ([]byte, error) {
	m, err := getMSIPath(json, "clusters", "value", clusterName, "kube_admin_config")
	if err != nil {
		return nil, err
	}

	host, err := get(m, "host")
	if err != nil {
		return nil, err
	}
	ca, err := get64(m, "cluster_ca_certificate")
	if err != nil && strings.HasPrefix(host, "https://") /*testenv host is just a plain IP address*/ {
		return nil, err
	}

	var ai clientcmdapi.AuthInfo
	b, err := get64(m, "client_certificate")
	if err == nil {
		ai.ClientCertificateData = b
	}
	b, err = get64(m, "client_key")
	if err == nil {
		ai.ClientKeyData = b
	}
	s, err := get(m, "username")
	if err == nil {
		ai.Username = s
	}
	s, err = get(m, "password")
	if err == nil {
		ai.Password = s
	}
	if !strings.HasPrefix(host, "127.0") {
		// API server on loopback adapter doesn't need auth.
		if (ai.ClientCertificateData == nil || ai.ClientKeyData == nil) && (ai.Username == "" || ai.Password == "") {
			return nil, fmt.Errorf("expected client_certificate,client_key or username,password")
		}
	}

	c := &clientcmdapi.Config{
		Clusters: []clientcmdapi.NamedCluster{
			{
				Name: clusterName,
				Cluster: clientcmdapi.Cluster{
					Server:                   host,
					CertificateAuthorityData: ca,
				},
			},
		},
		Contexts: []clientcmdapi.NamedContext{
			{
				Name: "default",
				Context: clientcmdapi.Context{
					Cluster:  clusterName,
					AuthInfo: "admin",
				},
			},
		},
		AuthInfos: []clientcmdapi.NamedAuthInfo{
			{
				Name:     "admin",
				AuthInfo: ai,
			},
		},
		CurrentContext: "default",
	}

	out, err := yaml.Marshal(c)
	if err != nil {
		return nil, err
	}

	return out, nil
}

func getMSIPath(data map[string]interface{}, keys ...string) (map[string]interface{}, error) {
	for _, k := range keys {
		v, ok := data[k]
		if !ok {
			return nil, fmt.Errorf("missing: %s", k)
		}
		data, ok = v.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("expected map: %s", k)
		}
	}
	return data, nil
}

func get(m map[string]interface{}, k string) (string, error) {
	v, ok := m[k]
	if !ok {
		return "", fmt.Errorf("missing: %s", k)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%s: expected string", k)
	}
	return s, nil
}

func get64(m map[string]interface{}, k string) ([]byte, error) {
	s, err := get(m, k)
	if err != nil {
		return []byte{}, err
	}
	d, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return []byte{}, fmt.Errorf("%s: %v", k, err)
	}
	return d, nil
}
