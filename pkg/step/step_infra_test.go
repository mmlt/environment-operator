package step

import (
	"encoding/json"
	"github.com/mmlt/environment-operator/pkg/cluster"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_kubeconfig(t *testing.T) {
	tests := []struct {
		it      string
		inJSON  string
		inName  string
		want    string
		wantErr string
	}{
		{
			it: "should create a kubeconfig from terraform output json",
			inJSON: `{
			  "client_certificate": "LS0tY2xpZW50X2NlcnRpZmljYXRl",
			  "client_key": "LS0tY2xpZW50X2tleQ==",
			  "cluster_ca_certificate": "LS0tY2xpZW50X2NhX2NlcnRpZmljYXRl",
			  "host": "https://env-xyz-123.hcp.northpole.azmk8s.io:443",
			  "password": "4ee5bb2b31",
			  "username": "clusterAdmin-rg_env-xyz"
			}`,
			inName: "xyz",
			want: `clusters:
- cluster:
    certificate-authority-data: LS0tY2xpZW50X2NhX2NlcnRpZmljYXRl
    server: https://env-xyz-123.hcp.northpole.azmk8s.io:443
  name: xyz
contexts:
- context:
    cluster: xyz
    user: admin
  name: default
current-context: default
preferences: {}
users:
- name: admin
  user:
    client-certificate-data: LS0tY2xpZW50X2NlcnRpZmljYXRl
    client-key-data: LS0tY2xpZW50X2tleQ==
    password: 4ee5bb2b31
    username: clusterAdmin-rg_env-xyz
`,
		},
		{
			it: "should create a kubeconfig when only username/password is provided",
			inJSON: `{
			  "cluster_ca_certificate": "LS0tY2xpZW50X2NhX2NlcnRpZmljYXRl",
			  "host": "https://env-xyz-123.hcp.northpole.azmk8s.io:443",
			  "password": "4ee5bb2b31",
			  "username": "clusterAdmin-rg_env-xyz"
			}`,
			inName: "xyz",
			want: `clusters:
- cluster:
    certificate-authority-data: LS0tY2xpZW50X2NhX2NlcnRpZmljYXRl
    server: https://env-xyz-123.hcp.northpole.azmk8s.io:443
  name: xyz
contexts:
- context:
    cluster: xyz
    user: admin
  name: default
current-context: default
preferences: {}
users:
- name: admin
  user:
    password: 4ee5bb2b31
    username: clusterAdmin-rg_env-xyz
`,
		},
		{
			it: "should error when an username without password is provided",
			inJSON: `{
			  "cluster_ca_certificate": "LS0tY2xpZW50X2NhX2NlcnRpZmljYXRl",
			  "host": "https://env-xyz-123.hcp.northpole.azmk8s.io:443",
			  "username": "clusterAdmin-rg_env-xyz"
			}`,
			inName:  "xyz",
			wantErr: "expected client_certificate,client_key or username,password",
		},
	}
	for _, tt := range tests {
		t.Run(tt.it, func(t *testing.T) {
			j := testUnmarshall(t, tt.inJSON)
			got, err := kubeconfig(j, tt.inName)
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.Equal(t, tt.wantErr, err.Error())
			}
			assert.Equal(t, tt.want, string(got))
		})
	}
}

func Test_clusters(t *testing.T) {
	tests := []struct {
		it      string
		inJSON  string
		inName  string
		want    []cluster.Cluster
		wantErr string
	}{
		{
			it: "should return clusters from terraform output json",
			inJSON: `{
  "clusters": {
    "sensitive": false,
    "type": [
      "object",
      {
        "cpe": [
          "object",
          {
            "kube_admin_config": [
              "object",
              {
                "client_certificate": "string",
                "client_key": "string",
                "cluster_ca_certificate": "string",
                "host": "string",
                "password": "string",
                "username": "string"
              }
            ]
          }
        ],
        "one": [
          "object",
          {
            "kube_admin_config": [
              "object",
              {
                "client_certificate": "string",
                "client_key": "string",
                "cluster_ca_certificate": "string",
                "host": "string",
                "password": "string",
                "username": "string"
              }
            ]
          }
        ]
      }
    ],
    "value": {
      "cpe": {
        "kube_admin_config": {
          "client_certificate": "eA==",
          "client_key": "eA==",
          "cluster_ca_certificate": "eA==",
          "host": "https://xxxd-cpe-xxx.hcp.westeurope.azmk8s.io:443",
          "password": "eA==",
          "username": "clusterAdmin_dxxxs_daksxxx-cpe"
        }
      },
      "one": {
        "kube_admin_config": {
          "client_certificate": "eA==",
          "client_key": "eA==",
          "cluster_ca_certificate": "eA==",
          "host": "https://xxxd-one-xxx.hcp.westeurope.azmk8s.io:443",
          "password": "eA==",
          "username": "clusterAdmin_dxxxs_daksxxx-one"
        }
      }
    }
  }
}
`,
			inName: "xyz",
			want: []cluster.Cluster{
				{Environment: "env", Name: "cpe", Domain: "dom", Provider: "prov", Config: []uint8{0x78}},
				{Environment: "env", Name: "one", Domain: "dom", Provider: "prov", Config: []uint8{0x78}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.it, func(t *testing.T) {
			j := testUnmarshall(t, tt.inJSON)
			got, err := clusters(j, "env", "dom", "prov")
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.Equal(t, tt.wantErr, err.Error())
			}

			// strip kc, we test them separately
			for k := range got {
				got[k].Config = []byte("x")
			}

			assert.Equal(t, tt.want, got)
		})
	}
}

func testUnmarshall(t *testing.T, in string) map[string]interface{} {
	t.Helper()
	out := map[string]interface{}{}
	err := json.Unmarshal([]byte(in), &out)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
