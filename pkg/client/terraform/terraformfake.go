package terraform

import (
	"context"
	"github.com/Jeffail/gabs/v2"
	"github.com/go-logr/logr"
	"os/exec"
	"time"
)

// TerraformFake provides a Terraformer for testing.
type TerraformFake struct {
	// Tally is the number of times Init, Plan, Apply has been called.
	InitTally, PlanTally, ApplyTally, DestroyTally, OutputTally, GetPlanTally int

	// Results that are returned by the fake implementations of Init or Plan.
	InitResult, PlanResult TFResult

	// Result that is played back by the fake implementation of StartApply.
	ApplyResult, DestroyResult []TFApplyResult

	// OutputResult is the parsed JSON output of terraform output.
	OutputResult map[string]interface{}

	// ShowPlanResult is the result of terraform show plan.
	ShowPlanResult string

	// Log
	Log logr.Logger
}

// Init implements Terraformer.
func (t *TerraformFake) Init(ctx context.Context, env []string, dir string) *TFResult {
	t.InitTally++
	return &t.InitResult
}

// Plan implements Terraformer.
func (t *TerraformFake) Plan(ctx context.Context, env []string, dir string) *TFResult {
	t.PlanTally++
	return &t.PlanResult
}

// StartApply implements Terraformer.
func (t *TerraformFake) StartApply(ctx context.Context, env []string, dir string) (*exec.Cmd, chan TFApplyResult, error) {
	t.ApplyTally++

	out := make(chan TFApplyResult)
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for _, v := range t.ApplyResult {
			select {
			case <-ticker.C:
				out <- v
			case <-ctx.Done():
				return
			}
		}
		close(out)
	}()

	return nil, out, nil
}

// StartDestroy implements Terraformer.
func (t *TerraformFake) StartDestroy(ctx context.Context, env []string, dir string) (*exec.Cmd, chan TFApplyResult, error) {
	t.DestroyTally++

	out := make(chan TFApplyResult)
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for _, v := range t.DestroyResult {
			select {
			case <-ticker.C:
				out <- v
			case <-ctx.Done():
				return
			}
		}
		close(out)
	}()

	return nil, out, nil
}

// Output implements Terraformer.
func (t *TerraformFake) Output(ctx context.Context, env []string, dir string) (map[string]interface{}, error) {
	t.OutputTally++
	return t.OutputResult, nil
}

// GetPlanPools reads an existing plan and returns AKSPools that are going to be updated or deleted.
func (t *TerraformFake) GetPlan(ctx context.Context, env []string, dir string) (*gabs.Container, error) {
	t.GetPlanTally++
	return gabs.ParseJSON([]byte(t.ShowPlanResult))
}

// SetupFakeResultsForCreate makes the fake replay a successful create.
// If clusters == nil it defaults to:
//	map[string]interface{}{
//		"mycluster": map[string]interface{}{
//			"kube_admin_config": map[string]interface{}{
//				"client_certificate":     string with base64 encoded value,
//				"client_key":             <idem>,
//				"cluster_ca_certificate": <idem>,
//				"host":                   "https://api.kubernetes.example.com:443",
//				"password":               "4ee5bb2",
//				"username":               "someadmin",
//			},
//		},
//	},
func (t *TerraformFake) SetupFakeResultsForCreate(clusters map[string]interface{}) {
	t.InitResult = TFResult{
		Info: 1,
	}

	t.PlanResult = TFResult{
		Info:        1,
		PlanAdded:   1,
		PlanChanged: 2,
		PlanDeleted: 1,
	}

	t.ApplyResult = []TFApplyResult{
		{Modifying: 1, Object: "azurerm_route_table.env", Action: "modifying", Elapsed: ""},
		{Modifying: 1, Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "destroying", Elapsed: ""},
		{Modifying: 1, Destroying: 1, Object: "azurerm_route_table.env", Action: "modifications", Elapsed: "1s"},
		{Modifying: 2, Destroying: 1, Object: "module.aks1.azurerm_subnet.this", Action: "modifying", Elapsed: ""},
		{Modifying: 2, Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "destroying", Elapsed: "10s"},
		{Modifying: 2, Destroying: 1, Object: "module.aks1.azurerm_subnet.this", Action: "modifying", Elapsed: "10s"},
		{Modifying: 2, Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "destroying", Elapsed: "20s"},
		{Modifying: 2, Destroying: 1, Object: "module.aks1.azurerm_subnet.this", Action: "modifying", Elapsed: "20s"},
		{Modifying: 2, Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "destroying", Elapsed: "30s"},
		{Modifying: 2, Destroying: 1, Object: "module.aks1.azurerm_subnet.this", Action: "modifying", Elapsed: "30s"},
		{Modifying: 2, Destroying: 1, Object: "module.aks1.azurerm_subnet.this", Action: "modifications", Elapsed: "32s"},
		{Modifying: 2, Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "destroying", Elapsed: "40s"},
		{Modifying: 2, Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "destroying", Elapsed: "50s"},
		{Modifying: 2, Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "destroying", Elapsed: "1m0s"},
		{Modifying: 2, Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "destruction", Elapsed: "1m8s"},
		{Creating: 1, Modifying: 2, Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "creating", Elapsed: ""},
		{Creating: 1, Modifying: 2, Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "creating", Elapsed: "[10s"},
		{Creating: 1, Modifying: 2, Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "creating", Elapsed: "[20s"},
		{Creating: 1, Modifying: 2, Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "creating", Elapsed: "[30s"},
		{Creating: 1, Modifying: 2, Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "creation", Elapsed: "6m22s"},
		{Creating: 1, Modifying: 2, Destroying: 1, TotalAdded: 1, TotalChanged: 2, TotalDestroyed: 1, Object: "", Action: "", Elapsed: ""}}

	t.ShowPlanResult = "{}"

	t.OutputResult = map[string]interface{}{
		"clusters": map[string]interface{}{
			"value": clusters,
		},
	}
}

// SetupFakeResultsForNothingToDo makes the fake replay a situation where terraform plan reports nothing to do.
func (t *TerraformFake) SetupFakeResultsForNothingToDo() {
	t.InitResult = TFResult{}

	t.PlanResult = TFResult{}

	t.ApplyResult = []TFApplyResult{}
}

// SetupFakeResultsForDeleteCluster makes the fake replay a situation where a cluster is being removed.
func (t *TerraformFake) SetupFakeResultsForDeleteCluster() {
	t.InitResult = TFResult{
		Info: 1,
	}

	t.PlanResult = TFResult{
		Info:        1,
		PlanDeleted: 1,
	}

	t.ApplyResult = []TFApplyResult{
		{Modifying: 1, Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "destroying", Elapsed: ""},
		{Modifying: 2, Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "destroying", Elapsed: "30s"},
		{Modifying: 2, Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "destroying", Elapsed: "1m0s"},
		{Modifying: 2, Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "destruction", Elapsed: "1m8s"},
		{Destroying: 1, TotalDestroyed: 1, Object: "", Action: "", Elapsed: ""}}

	// note that the following is only a fragment of a 'terraform plan' result,
	// see pkg/client/terraform/testdata for complete examples"
	t.ShowPlanResult = `{
  "resource_changes": [
    {
      "address": "module.aks2.azurerm_kubernetes_cluster.this",
      "module_address": "module.aks2",
      "mode": "managed",
      "type": "azurerm_kubernetes_cluster",
      "name": "this",
      "provider_name": "registry.terraform.io/hashicorp/azurerm",
      "change": {
        "actions": [
          "delete"
        ],
        "before": {
          "id": "/subscriptions/ea-xx-xx-xx-5/resourceGroups/srgr002k8s/providers/Microsoft.ContainerService/managedClusters/xyz",
          "kube_admin_config_raw": "apiVersion: v1\nclusters:\n- cluster:\n    certificate-authority-data: LS0tL==\n    server: https://k8s.example.com:443\n  name: xyz\ncontexts:\n- context:\n    cluster: xyz\n    user: clusterAdmin_xyz\n  name: xyz\ncurrent-context: xyz\nkind: Config\npreferences: {}\nusers:\n- name: clusterAdmin_xyz\n  user:\n    client-certificate-data: LS0t==\n    client-key-data: LS0K\n    token: 1c8\n"
        }
      },
      "action_reason": "delete_because_no_resource_config"
    }
  ]
}`

	t.OutputResult = map[string]interface{}{
		"clusters": map[string]interface{}{
			"value": map[string]interface{}{},
		},
	}
}

// SetupFakeResultsForFailedDestroy makes the fake replay failed destroy.
func (t *TerraformFake) SetupFakeResultsForFailedDestroy() {
	t.DestroyResult = nil
}

// SetupFakeResultsForSuccessfulDestroy makes the fake replay a successful destroy.
func (t *TerraformFake) SetupFakeResultsForSuccessfulDestroy() {
	t.DestroyResult = []TFApplyResult{
		{Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "destroying", Elapsed: "40s"},
		{Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "destroying", Elapsed: "50s"},
		{Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "destroying", Elapsed: "1m0s"},
		{Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "destruction", Elapsed: "1m8s"},
		{Destroying: 1, TotalAdded: 0, TotalChanged: 0, TotalDestroyed: 1, Object: "", Action: "", Elapsed: ""}}
}
