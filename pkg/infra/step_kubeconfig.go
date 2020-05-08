package infra

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/mmlt/environment-operator/pkg/terraform"
	"io/ioutil"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api/v1"
	"sigs.k8s.io/yaml"
)

// InitStep performs a terraform init
type KubeconfigStep struct {
	StepMeta

	/* Parameters */

	// TFPath is the path to the directory containing terraform code.
	TFPath string
	// ClusterName is the name of the k8s cluster to create a kube config for.
	ClusterName string
	// KCPath is the place were the kube config file is written.
	KCPath string

	// Hash is an opaque value passed to Update.
	Hash string //TODO not needed (we run this step before AddonStep)
}

// Meta returns a reference to the meta data this Step.
func (st *KubeconfigStep) Meta() *StepMeta {
	return &st.StepMeta
}

// Type returns the type of this Step.
func (st *KubeconfigStep) Type() string {
	return "ClusterKubeconfig"
}

/*// ID returns a unique identification of this step.
func (st *InitStep) id() StepID {
	return st.ID
}*/

/*// Ord returns the execution order of this step.
func (st *KubeconfigStep) ord() StepOrd {
	return StepOrdInit
}*/

// Run a step.
func (st *KubeconfigStep) execute(ctx context.Context, isink Infoer, usink Updater, tf terraform.Terraformer /*TODO remove*/, log logr.Logger) bool {
	log.Info("KubeconfigStep")

	// Run.
	st.State = StepStateRunning
	usink.Update(st)

	o, err := tf.Output(st.TFPath)
	if err != nil {
		st.State = StepStateError
		st.Msg = fmt.Sprintf("terraform output: %v", err)
		usink.Update(st)
		return false
	}

	kc, err := kubeconfig(o, st.ClusterName)
	if err != nil {
		st.State = StepStateError
		st.Msg = fmt.Sprintf("kubeconfig from terraform output: %v", err)
		usink.Update(st)
		return false
	}

	ioutil.WriteFile(st.KCPath, kc, 0664)
	if err != nil {
		st.State = StepStateError
		st.Msg = fmt.Sprintf("write kubeconfig: %v", err)
		usink.Update(st)
		return false
	}

	// Return results.
	st.State = StepStateReady
	// TODO return values

	usink.Update(st)

	return st.State == StepStateReady
}

func kubeconfig(json map[string]interface{}, clusterName string) ([]byte, error) {
	m, err := getMSIPath(json, "clusters", "value", clusterName, "kube_admin_config")
	if err != nil {
		return nil, err
	}

	get := func(k string) string {
		if err != nil {
			return ""
		}
		v, ok := m[k]
		if !ok {
			err = fmt.Errorf("missing: %s", k)
		}
		s, ok := v.(string)
		if !ok {
			err = fmt.Errorf("%s: expected string", k)
		}
		return s
	}
	get64 := func(k string) []byte {
		s := get(k)
		if err != nil {
			return []byte{}
		}
		d, e := base64.StdEncoding.DecodeString(s)
		if e != nil {
			err = fmt.Errorf("%s: %v", k, e)
		}
		return d
	}

	c := &clientcmdapi.Config{
		Clusters: []clientcmdapi.NamedCluster{
			{
				Name: clusterName,
				Cluster: clientcmdapi.Cluster{
					Server:                   get("host"),
					CertificateAuthorityData: get64("cluster_ca_certificate"),
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
				Name: "admin",
				AuthInfo: clientcmdapi.AuthInfo{
					ClientCertificateData: get64("client_certificate"),
					ClientKeyData:         get64("client_key"),
				},
			},
		},
		CurrentContext: "default",
	}

	if err != nil {
		return nil, err
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
