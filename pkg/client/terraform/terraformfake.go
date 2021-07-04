package terraform

import (
	"context"
	"github.com/go-logr/logr"
	"os/exec"
	"time"
)

// TerraformFake provides a Terraformer for testing.
type TerraformFake struct {
	// Tally is the number of times Init, Plan, Apply has been called.
	InitTally, PlanTally, ApplyTally, DestroyTally, OutputTally, GetPlanPoolsTally int

	// Results that are returned by the fake implementations of Init or Plan.
	InitResult, PlanResult TFResult

	// Result that is played back by the fake implementation of StartApply.
	ApplyResult, DestroyResult []TFApplyResult

	// OutputResult is the parsed JSON output of terraform output.
	OutputResult map[string]interface{}

	// GetPlanPoolsResult is the result of GetPlanPools().
	GetPlanPoolsResult []AKSPool

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
func (t *TerraformFake) GetPlanPools(ctx context.Context, env []string, dir string) ([]AKSPool, error) {
	t.GetPlanPoolsTally++
	return t.GetPlanPoolsResult, nil
}

// SetupFakeResults sets-up the receiver with data that is returned during testing.
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
func (t *TerraformFake) SetupFakeResults(clusters map[string]interface{}) {
	if clusters == nil {
		clusters = map[string]interface{}{
			"mycluster": map[string]interface{}{
				"kube_admin_config": map[string]interface{}{
					"client_certificate":     "Y2xpZW50X2NlcnRpZmljYXRl",
					"client_key":             "Y2xpZW50X2tleQ==",
					"cluster_ca_certificate": "Y2x1c3Rlcl9jYV9jZXJ0aWZpY2F0ZQ==",
					"host":                   "https://api.kubernetes.example.com:443",
					"password":               "4ee5bb2",
					"username":               "someadmin",
				},
			},
		}
	}

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

	t.DestroyMustSucceed()

	t.OutputResult = map[string]interface{}{
		"clusters": map[string]interface{}{
			"value": clusters,
		},
	}
}

// DestroyMustSucceed makes the fake replay failed destroy.
func (t *TerraformFake) DestroyMustFail() {
	t.DestroyResult = nil
}

// DestroyMustSucceed makes the fake replay a successful destroy.
func (t *TerraformFake) DestroyMustSucceed() {
	t.DestroyResult = []TFApplyResult{
		{Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "destroying", Elapsed: "40s"},
		{Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "destroying", Elapsed: "50s"},
		{Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "destroying", Elapsed: "1m0s"},
		{Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "destruction", Elapsed: "1m8s"},
		{Destroying: 1, TotalAdded: 0, TotalChanged: 0, TotalDestroyed: 1, Object: "", Action: "", Elapsed: ""}}
}
