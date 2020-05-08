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
	InitTally, PlanTally, ApplyTally, OutputTally int

	// Results that are returned by the fake implementations of Init or Plan.
	InitResult, PlanResult TFResult

	// Result that is played back by the fake implementation of StartApply.
	ApplyResult []TFApplyResult

	// OutputResult is the parsed JSON output of terraform output.
	OutputResult map[string]interface{}

	// Log
	Log logr.Logger
}

func NewFake(log logr.Logger) *TerraformFake {
	return &TerraformFake{
		InitResult: TFResult{
			Info: 1,
		},
		PlanResult: TFResult{
			Info: 1,
		},
		ApplyResult: []TFApplyResult{
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
			{Creating: 1, Modifying: 2, Destroying: 1, TotalAdded: 1, TotalChanged: 2, TotalDestroyed: 1, Object: "", Action: "", Elapsed: ""}},
	}
}

// Init implements Terraformer.
func (t *TerraformFake) Init(dir string) *TFResult {
	t.InitTally++
	return &t.InitResult
}

// Plan implements Terraformer.
func (t *TerraformFake) Plan(dir string) *TFResult {
	t.PlanTally++
	return &t.PlanResult
}

// StartApply implements Terraformer.
func (t *TerraformFake) StartApply(ctx context.Context, dir string) (*exec.Cmd, chan TFApplyResult, error) {
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

// Output implements Terraformer.
func (t *TerraformFake) Output(dir string) (map[string]interface{}, error) {
	t.OutputTally++
	return t.OutputResult, nil
}

// SetupFakeResults sets the receiver up data that is returned during testing.
func (t *TerraformFake) SetupFakeResults() {
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

	t.OutputResult = map[string]interface{}{
		"clusters": map[string]interface{}{
			"value": map[string]interface{}{
				"mycluster": map[string]interface{}{
					"kube_admin_config": map[string]interface{}{
						"client_certificate":     "LS0tLS1Cclientcert",
						"client_key":             "LS0tLS1CRclientkey",
						"cluster_ca_certificate": "LS0tLS1CRcacert",
						"host":                   "https://xy-clustername-123a.hcp.westeurope.azmk8s.io:443",
						"password":               "4ee5bb2",
						"username":               "clusterAdmin_aaa-rg_xy-clustername",
					},
				},
			},
		},
	}
}
