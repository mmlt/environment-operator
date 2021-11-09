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
	"github.com/mmlt/environment-operator/pkg/cluster"
	"github.com/mmlt/environment-operator/pkg/util"
	"github.com/mmlt/environment-operator/pkg/util/backoff"
	"io/ioutil"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api/v1"
	"sigs.k8s.io/yaml"
	"strings"
	"time"
)

// KubeconfigStep reads data from terraform, creates a kubeconfig file and syncs Secrets containing kubeconfigs.
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

	// Values are key-values like k8sEnvironment, k8sCluster, k8sDomain, k8sProvider
	Values map[string]string

	// Client is used to access the cluster envop is running in.
	Client cluster.Client
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

	err = st.syncClusterSecrets(o)
	if err != nil {
		st.error2(err, "sync cluster secrets")
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

// SyncClusterSecrets creates/updates/deletes cluster Secrets to match Terraform json output.
func (st *KubeconfigStep) syncClusterSecrets(json map[string]interface{}) error {
	ctx := context.TODO()

	// desired state
	m, err := getMSIPath(json, "clusters", "value")
	if err != nil {
		return err
	}

	desired := []cluster.Cluster{}
	for n, _ := range m {
		kc, err := kubeconfig(json, n)
		if err != nil {
			return err
		}

		if v := st.Values["k8sCluster"]; n != v {
			return fmt.Errorf("cluster name '%s' should equal k8sCluster value '%s'", n, v)
		}

		desired = append(desired, cluster.Cluster{
			Environment: st.Values["k8sEnvironment"],
			Name:        st.Values["k8sCluster"],
			Domain:      st.Values["k8sDomain"],
			Provider:    st.Values["k8sProvider"],
			Config:      kc,
		})
	}

	// current state
	current, err := st.Client.List(ctx, st.Metaa.ID.Namespace)
	if err != nil {
		return err
	}

	c, u, d := cluster.Diff(current, desired)

	err = st.Client.Create(ctx, st.Metaa.ID.Namespace, c)
	if err != nil {
		return err
	}
	err = st.Client.Update(ctx, st.Metaa.ID.Namespace, u)
	if err != nil {
		return err
	}
	err = st.Client.Delete(ctx, st.Metaa.ID.Namespace, d)
	if err != nil {
		return err
	}

	return nil
}

// Kubeconfig returns a kube config from Terraform json output.
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

// GetMSIPath returns the subtree at keys path in data.
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

// Get returns a string value at m[k]
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

// Get returns a base 64 encoded []byte value at m[k]
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
