package step

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/go-logr/logr"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/client/terraform"
	"io/ioutil"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api/v1"
	"sigs.k8s.io/yaml"
	"strings"
)

// InitStep performs a terraform init
type KubeconfigStep struct {
	meta

	/* Parameters */

	// TFPath is the path to the directory containing terraform code.
	TFPath string
	// ClusterName is the name of the k8s cluster to create a kube config for.
	ClusterName string
	// KCPath is the place were the kube config file is written.
	KCPath string
}

// Meta returns a reference to the meta data this Step.
func (st *KubeconfigStep) Meta() *meta {
	return &st.meta
}

// Run a step.
func (st *KubeconfigStep) Execute(ctx context.Context, isink Infoer, usink Updater, tf terraform.Terraformer /*TODO remove*/, log logr.Logger) bool {
	log.Info("start")

	// Run.
	st.State = v1.StateRunning
	usink.Update(st)

	o, err := tf.Output(st.TFPath)
	if err != nil {
		st.State = v1.StateError
		st.Msg = fmt.Sprintf("terraform output: %v", err)
		usink.Update(st)
		return false
	}

	kc, err := kubeconfig(o, st.ClusterName)
	if err != nil {
		st.State = v1.StateError
		st.Msg = fmt.Sprintf("kubeconfig from terraform output: %v", err)
		usink.Update(st)
		return false
	}

	ioutil.WriteFile(st.KCPath, kc, 0664)
	if err != nil {
		st.State = v1.StateError
		st.Msg = fmt.Sprintf("write kubeconfig: %v", err)
		usink.Update(st)
		return false
	}

	// Return results.
	st.State = v1.StateReady
	// TODO return values

	usink.Update(st)

	return st.State == v1.StateReady
}

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
