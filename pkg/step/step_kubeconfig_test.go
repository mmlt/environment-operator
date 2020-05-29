package step

import (
	"encoding/json"
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
  "clusters": {
    "sensitive": false,
    "type": [
      "object",
      {
        "xyz": [
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
    "value": {
      "xyz": {
        "kube_admin_config": {
          "client_certificate": "LS0tY2xpZW50X2NlcnRpZmljYXRl",
          "client_key": "LS0tY2xpZW50X2tleQ==",
          "cluster_ca_certificate": "LS0tY2xpZW50X2NhX2NlcnRpZmljYXRl",
          "host": "https://env-xyz-123.hcp.northpole.azmk8s.io:443",
          "password": "4ee5bb2b31",
          "username": "clusterAdmin-rg_env-xyz"
        }
      }
    }
  }
}
`,
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
  "clusters": {
    "sensitive": false,
    "type": [
      "object",
      {
        "xyz": [
          "object",
          {
            "cluster_ca_certificate": "string",
            "host": "string",
            "password": "string",
            "username": "string"
          }
        ]
      }
    ],
    "value": {
      "xyz": {
        "kube_admin_config": {
          "cluster_ca_certificate": "LS0tY2xpZW50X2NhX2NlcnRpZmljYXRl",
          "host": "https://env-xyz-123.hcp.northpole.azmk8s.io:443",
          "password": "4ee5bb2b31",
          "username": "clusterAdmin-rg_env-xyz"
        }
      }
    }
  }
}
`,
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
  "clusters": {
    "sensitive": false,
    "type": [
      "object",
      {
        "xyz": [
          "object",
          {
            "cluster_ca_certificate": "string",
            "host": "string",
            "password": "string",
            "username": "string"
          }
        ]
      }
    ],
    "value": {
      "xyz": {
        "kube_admin_config": {
          "cluster_ca_certificate": "LS0tY2xpZW50X2NhX2NlcnRpZmljYXRl",
          "host": "https://env-xyz-123.hcp.northpole.azmk8s.io:443",
          "username": "clusterAdmin-rg_env-xyz"
        }
      }
    }
  }
}
`,
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

func testUnmarshall(t *testing.T, in string) map[string]interface{} {
	out := map[string]interface{}{}
	err := json.Unmarshal([]byte(in), &out)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
