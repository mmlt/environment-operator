package terraform

import (
	"github.com/Jeffail/gabs/v2"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"path/filepath"
	"testing"
)

func Test_ClustersFromPlan(t *testing.T) {
	//t.Skip("WIP; needs plan.json file")

	b, err := ioutil.ReadFile(filepath.Join("testdata", "plan.json"))
	assert.NoError(t, err)
	json, err := gabs.ParseJSON(b)
	assert.NoError(t, err)

	// cat pkg/client/terraform/testdata/plan.json | jq '.resource_changes[] | select(.type == "azurerm_kubernetes_cluster")' | more
	want := []AKSCluster{
		{ResourceGroup: "srgr001k8s", Cluster: "saks001eu99y-cpe", Action: ActionDelete, KubeconfigRaw: "REDACTED"},
	}

	got, err := ClustersFromPlan(json)
	if assert.NoError(t, err) {
		assert.Equal(t, want, got)
	}
}

func Test_PoolsFromPlan(t *testing.T) {
	//t.Skip("WIP; needs plan.json file")

	b, err := ioutil.ReadFile(filepath.Join("testdata", "plan.json"))
	assert.NoError(t, err)
	json, err := gabs.ParseJSON(b)
	assert.NoError(t, err)

	// cat pkg/client/terraform/testdata/plan.json | jq '.resource_changes[] | select(.type == "azurerm_kubernetes_cluster_node_pool")' | more
	want := []AKSPool{
		{ResourceGroup: "srgr001k8s", Cluster: "saks001eu99y-cpe", Pool: "extra", MinCount: 1, MaxCount: 10, Action: ActionUpdate},
		{ResourceGroup: "srgr001k8s", Cluster: "saks001eu99y-cpe", Pool: "extra1", MinCount: 1, MaxCount: 10, Action: ActionDelete},
		{ResourceGroup: "srgr001k8s", Cluster: "saks001eu99y-cpe", Pool: "extra2", MinCount: 1, MaxCount: 10, Action: ActionDelete},
		{ResourceGroup: "srgr001k8s", Cluster: "saks001eu99y-cpe", Pool: "extra3", MinCount: 1, MaxCount: 10, Action: ActionDelete},
	}
	got, err := PoolsFromPlan(json)
	if assert.NoError(t, err) {
		assert.Equal(t, want, got)
	}
}

func Test_pathToMap(t *testing.T) {
	tests := []struct {
		it      string
		in      string
		want    map[string]string
		wantErr bool
	}{
		{
			it: "should handle input with proper key/value pairs",
			in: "/subscriptions/ea-xx-xx-xx-5/resourcegroups/srgr001k8s/managedClusters/saks001eu99y-cpe/agentPools/extra",
			want: map[string]string{
				"agentpools":      "extra",
				"managedclusters": "saks001eu99y-cpe",
				"resourcegroups":  "srgr001k8s",
				"subscriptions":   "ea-xx-xx-xx-5",
			},
		},
		{
			it:      "should error on odd number of elements input with proper key/value pairs",
			in:      "/key",
			wantErr: true,
		},
		{
			it:      "should error on empty input",
			in:      "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.it, func(t *testing.T) {
			got, err := pathToMap(tt.in)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				if assert.NoError(t, err) {
					assert.Equal(t, tt.want, got)
				}
			}
		})
	}
}
