package terraform

import (
	"github.com/mmlt/testr"
	"github.com/stretchr/testify/assert"
	"io"
	"os/exec"
	"strconv"
	"testing"
)

func TestParseInitResponse(t *testing.T) {
	tsts := []struct {
		it    string
		in    string
		inErr error
		want  TFResult
	}{
		{
			it: "must return success for first terraform init",
			in: `Initializing modules...
- test in modules/aks

Initializing the backend...

Initializing provider plugins...
- Checking for available provider plugins...
- Downloading plugin for provider "azuread" (hashicorp/azuread) 0.3.1...
- Downloading plugin for provider "azurerm" (hashicorp/azurerm) 1.39.0...

Terraform has been successfully initialized!

You may now begin working with Terraform. Try running "terraform plan" to see
any changes that are required for your infrastructure. All Terraform commands
should now work.

If you ever set or change modules or backend configuration for Terraform,
rerun this command to reinitialize your working directory. If you forget, other
commands will detect it and remind you to do so if necessary.
`,
			inErr: nil,
			want: TFResult{
				Info: 1,
				Text: "Initializing modules...\n- test in modules/aks\n\nInitializing the backend...\n\nInitializing provider plugins...\n- Checking for available provider plugins...\n- Downloading plugin for provider \"azuread\" (hashicorp/azuread) 0.3.1...\n- Downloading plugin for provider \"azurerm\" (hashicorp/azurerm) 1.39.0...\n\nTerraform has been successfully initialized!\n\nYou may now begin working with Terraform. Try running \"terraform plan\" to see\nany changes that are required for your infrastructure. All Terraform commands\nshould now work.\n\nIf you ever set or change modules or backend configuration for Terraform,\nrerun this command to reinitialize your working directory. If you forget, other\ncommands will detect it and remind you to do so if necessary.\n",
			},
		},
		{
			it: "must return success for when terraform init is repeated",
			in: `Initializing modules...

Initializing the backend...

Initializing provider plugins...

Terraform has been successfully initialized!

You may now begin working with Terraform. Try running "terraform plan" to see
any changes that are required for your infrastructure. All Terraform commands
should now work.

If you ever set or change modules or backend configuration for Terraform,
rerun this command to reinitialize your working directory. If you forget, other
commands will detect it and remind you to do so if necessary.
`,
			inErr: nil,
			want: TFResult{
				Info: 1,
				Text: "Initializing modules...\n\nInitializing the backend...\n\nInitializing provider plugins...\n\nTerraform has been successfully initialized!\n\nYou may now begin working with Terraform. Try running \"terraform plan\" to see\nany changes that are required for your infrastructure. All Terraform commands\nshould now work.\n\nIf you ever set or change modules or backend configuration for Terraform,\nrerun this command to reinitialize your working directory. If you forget, other\ncommands will detect it and remind you to do so if necessary.\n",
			},
		},
		{
			it: "must error when terraform init dir does not exist",
			in: `Terraform initialized in an empty directory!

The directory has no Terraform configuration files. You may begin working
with Terraform immediately by creating Terraform configuration files.
`,
			inErr: nil,
			want: TFResult{
				Text: "Terraform initialized in an empty directory!\n\nThe directory has no Terraform configuration files. You may begin working\nwith Terraform immediately by creating Terraform configuration files.\n",
			},
		},
		{
			it: "must error when 'provider' has typos",
			in: `Initializing modules...

Initializing provider plugins...

The following providers do not have any version constraints in configuration,
so the latest version was installed.

To prevent automatic upgrades to new major versions that may contain breaking
changes, it is recommended to add version = "..." constraints to the
corresponding provider blocks in configuration, with the constraint strings
suggested below.

* provider.azurerm: version = "~> 1.39"


Warning: Skipping backend initialization pending configuration upgrade

The root module configuration contains errors that may be fixed by running the
configuration upgrade tool, so Terraform is skipping backend initialization.
See below for more information.


Terraform has initialized, but configuration upgrades may be needed.

Terraform found syntax errors in the configuration that prevented full
initialization. If you've recently upgraded to Terraform v0.12, this may be
because your configuration uses syntax constructs that are no longer valid,
and so must be updated before full initialization is possible.

Terraform has installed the required providers to support the configuration
upgrade process. To begin upgrading your configuration, run the following:
    terraform 0.12upgrade

To see the full set of errors that led to this message, run:
    terraform validate
`,
			inErr: nil,
			want: TFResult{
				Warnings: 1,
				Text:     "Initializing modules...\n\nInitializing provider plugins...\n\nThe following providers do not have any version constraints in configuration,\nso the latest version was installed.\n\nTo prevent automatic upgrades to new major versions that may contain breaking\nchanges, it is recommended to add version = \"...\" constraints to the\ncorresponding provider blocks in configuration, with the constraint strings\nsuggested below.\n\n* provider.azurerm: version = \"~> 1.39\"\n\n\nWarning: Skipping backend initialization pending configuration upgrade\n\nThe root module configuration contains errors that may be fixed by running the\nconfiguration upgrade tool, so Terraform is skipping backend initialization.\nSee below for more information.\n\n\nTerraform has initialized, but configuration upgrades may be needed.\n\nTerraform found syntax errors in the configuration that prevented full\ninitialization. If you've recently upgraded to Terraform v0.12, this may be\nbecause your configuration uses syntax constructs that are no longer valid,\nand so must be updated before full initialization is possible.\n\nTerraform has installed the required providers to support the configuration\nupgrade process. To begin upgrading your configuration, run the following:\n    terraform 0.12upgrade\n\nTo see the full set of errors that led to this message, run:\n    terraform validate\n",
			},
		},
		{
			it:    "it must error when terraform init exits with non-zero code",
			in:    ``,
			inErr: newExitError(1),
			want: TFResult{
				Errors: []string{
					"exec: \"exit\": executable file not found in $PATH",
				},
			},
		},
		{
			it:    "must handle empty input",
			in:    ``,
			inErr: nil,
			want:  TFResult{},
		},
	}

	for _, tst := range tsts {
		t.Run(tst.it, func(t *testing.T) {
			got := parseInitResponse(tst.in, tst.inErr)
			assert.Equal(t, tst.want, *got)
		})
	}
}

// sameAsIn is used when the string is the same as the tst.in value.
const sameAsIn = "SAME_AS_IN"

func TestParsePlanResponse(t *testing.T) {
	tsts := []struct {
		it    string
		in    string
		inErr error
		want  TFResult
	}{
		{
			it:    "must error when terraform plan is invoked with a non existing dir",
			in:    `stat _main.tf: no such file or directory`,
			inErr: newExitError(1),
			want: TFResult{
				Text: sameAsIn,
			},
		},
		{
			it: "must error when terraform plan is invoked with a non existing -tfvars-file",
			in: `
Error: Failed to read variables file

Given variables file _main.tfvars does not exist.
`,
			inErr: nil,
			want: TFResult{
				Text: sameAsIn,
			},
		},
		{
			it: "must warn when values are provided that aren't used",
			in: `
Warning: Value for undeclared variable

The root module does not declare a variable named "resource_group_name" but a
value was found in file "_main.tfvars". To use this value, add a "variable"
block to the configuration.

Using a variables file to set an undeclared variable is deprecated and will
become an error in a future release. If you wish to provide certain "global"
settings to all configurations in your organization, use TF_VAR_...
environment variables to set these instead.
`,
			inErr: nil,
			want: TFResult{
				Warnings: 1,
				Text:     sameAsIn,
			},
		},
		{
			it: "must return success if no errors are present",
			in: `
Refreshing Terraform state in-memory prior to plan...
The refreshed state will be used to calculate this plan, but will not be
persisted to local or remote state storage.

data.azurerm_resource_group.env: Refreshing state...

------------------------------------------------------------------------

An execution plan has been generated and is shown below.
Resource actions are indicated with the following symbols:
  + create

Terraform will perform the following actions:

  # azuread_application.vnet-sp will be created
  + resource "azuread_application" "vnet-sp" {
      + application_id             = (known after apply)
      + available_to_other_tenants = false
      + homepage                   = (known after apply)
      + id                         = (known after apply)
      + identifier_uris            = (known after apply)
      + name                       = "yyy-vnet-sp"
      + reply_urls                 = (known after apply)
    }

  # azuread_service_principal.vnet-sp will be created
  + resource "azuread_service_principal" "vnet-sp" {
      + application_id = (known after apply)
      + display_name   = (known after apply)
      + id             = (known after apply)
    }

  # module.test.azurerm_subnet_route_table_association.this will be created
  + resource "azurerm_subnet_route_table_association" "this" {
      + id             = (known after apply)
      + route_table_id = (known after apply)
      + subnet_id      = (known after apply)
    }

Plan: 3 to add, 0 to change, 0 to destroy.
`,
			inErr: nil,
			want: TFResult{
				Info:      1,
				PlanAdded: 3,
				Text:      sameAsIn,
			},
		},
		{
			it: "must handle backticks and double quotes",
			in: "\n" +
				"Warning: \"route_table_id\": [DEPRECATED] Use the `azurerm_subnet_route_table_association` resource instead.\n" +
				"\n" +
				"  on main.tf line 39, in resource \"azurerm_subnet\" \"intlb\":\n" +
				"  39: resource \"azurerm_subnet\" \"intlb\" {\"\n" +
				"\n" +
				"(and one more similar warning elsewhere)\n",
			inErr: nil,
			want: TFResult{
				Warnings: 1,
				Text:     sameAsIn,
			},
		},
		{
			it:    "must error when terraform plan exits with exit code 1",
			in:    ``,
			inErr: newExitError(1),
			want: TFResult{
				Text: sameAsIn,
			},
		},
		{
			it:    "must handle empty input",
			in:    ``,
			inErr: nil,
			want:  TFResult{},
		},
		{
			it: "must parse the numbers to add, change, delete correctly",
			in: `<some input deleted>

Plan: 1 to add, 22 to change, 33 to destroy.

------------------------------------------------------------------------

This plan was saved to: newplan

To perform exactly these actions, run the following command to apply:
    terraform apply "newplan"
`,
			inErr: nil,
			want: TFResult{
				Info:        1,
				PlanAdded:   1,
				PlanChanged: 22,
				PlanDeleted: 33,
				Text:        sameAsIn,
			},
		},
	}

	for _, tst := range tsts {
		t.Run(tst.it, func(t *testing.T) {
			got := parsePlanResponse(tst.in, tst.inErr)
			if tst.want.Text == sameAsIn {
				tst.want.Text = tst.in
			}
			assert.Equal(t, tst.want, *got)
		})
	}
}

func TestParseAsyncApplyResponse(t *testing.T) {
	tsts := []struct {
		it   string
		in   []string
		want []TFApplyResult
	}{
		{
			it: "must error when not authorized",
			in: []string{
				"azuread_application.vnet-sp: Creating...\n",
				"azuread_application.vnet-sp: Creation complete after 0s [id=23...]\n",
				"azurerm_route_table.env: Creating...\n",
				"azurerm_virtual_network.env: Creating...\n",
				"\nError: Error Creating/Updating Route Table \"routetable\" (Resource Group \"test-rg\"): network.RouteTablesClient#CreateOrUpdate: Failure sending request: StatusCode=403 -- Original Error: Code=\"AuthorizationFailed\" Message=\"The client 'xyz@example.com' with object id '79..' does not have authorization to perform action 'Microsoft.Network/routeTables/write' over scope '/subscriptions/ea../resourceGroups/test-rg/providers/Microsoft.Network/routeTables/test-routetable' or the scope is invalid. If access was recently granted, please refresh your credentials.\n",
				"  on main.tf line 20, in resource \"azurerm_route_table\" \"env\":\n  20: resource \"azurerm_route_table\" \"env\" {\n",
				"\n"},
			want: []TFApplyResult{
				{Creating: 1, Object: "azuread_application.vnet-sp", Action: "creating"},
				{Creating: 1, Object: "azuread_application.vnet-sp", Action: "creation", Elapsed: "0s",
					Text: "azuread_application.vnet-sp: Creation complete after 0s [id=23...]",
				},
				{Creating: 2, Object: "azurerm_route_table.env", Action: "creating"},
				{Creating: 3, Object: "azurerm_virtual_network.env", Action: "creating"},
				{Creating: 3,
					Errors: []string{
						"Error Creating/Updating Route Table \"routetable\" (Resource Group \"test-rg\"): network.RouteTablesClient#CreateOrUpdate: Failure sending request: StatusCode=403 -- Original Error: Code=\"AuthorizationFailed\" Message=\"The client 'xyz@example.com' with object id '79..' does not have authorization to perform action 'Microsoft.Network/routeTables/write' over scope '/subscriptions/ea../resourceGroups/test-rg/providers/Microsoft.Network/routeTables/test-routetable' or the scope is invalid. If access was recently granted, please refresh your credentials.",
					},
					Text: "Error: Error Creating/Updating Route Table \"routetable\" (Resource Group \"test-rg\"): network.RouteTablesClient#CreateOrUpdate: Failure sending request: StatusCode=403 -- Original Error: Code=\"AuthorizationFailed\" Message=\"The client 'xyz@example.com' with object id '79..' does not have authorization to perform action 'Microsoft.Network/routeTables/write' over scope '/subscriptions/ea../resourceGroups/test-rg/providers/Microsoft.Network/routeTables/test-routetable' or the scope is invalid. If access was recently granted, please refresh your credentials.",
				},
			},
		},
		{
			it: "must error on Error: msg input",
			in: []string{
				"module.aks1.azurerm_kubernetes_cluster.this: Creating...\n",
				"\n",
				"Error: A resource with the ID \"/subscriptions/ea365/resourcegroups/xxx-rg/providers/Microsoft.ContainerService/managedClusters/yyy-cpe\" already exists - to be managed via Terraform this resource needs to be imported into the State. Please see the resource documentation for \"azurerm_kubernetes_cluster\" for more information.\n",
				"\n",
				//TODO fix: this line should be in the output because it is needed for 'terraform import'
				"  on modules/aks/main.tf line 16, in resource \"azurerm_kubernetes_cluster\" \"this\":\n",
				"  16: resource \"azurerm_kubernetes_cluster\" \"this\" {\n",
				"\n",
			},
			want: []TFApplyResult{
				{Creating: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "creating"},
				{Creating: 1,
					Errors: []string{"A resource with the ID \"/subscriptions/ea365/resourcegroups/xxx-rg/providers/Microsoft.ContainerService/managedClusters/yyy-cpe\" already exists - to be managed via Terraform this resource needs to be imported into the State. Please see the resource documentation for \"azurerm_kubernetes_cluster\" for more information."},
					Text:   "Error: A resource with the ID \"/subscriptions/ea365/resourcegroups/xxx-rg/providers/Microsoft.ContainerService/managedClusters/yyy-cpe\" already exists - to be managed via Terraform this resource needs to be imported into the State. Please see the resource documentation for \"azurerm_kubernetes_cluster\" for more information.",
				},
			},
		},
		{
			it: "must error on Error with StatusCode=400",
			in: []string{
				"module.aks1.azurerm_subnet.this: Destroying... [id=/subscriptions/ea365/resourceGroups/xxx-rg/providers/Microsoft.Network/virtualNetworks/yyy-vnet/subnets/cpe]\n",
				"module.aks1.azurerm_subnet.this: Destruction complete after 0s\n",
				"\n",
				"Error: Error deleting Subnet \"intlb\" (Virtual Network \"yyy-vnet\" / Resource Group \"xxx-rg\"): network.SubnetsClient#Delete: Failure sending request: StatusCode=400 -- Original Error: Code=\"InUseSubnetCannotBeDeleted\" Message=\"Subnet intlb is in use by /subscriptions/ea365/resourceGroups/mc_xxx-rg_x_westeurope/providers/Microsoft.Network/loadBalancers/kubernetes-internal/frontendIPConfigurations/a57-intlb and cannot be deleted. In order to delete the subnet, delete all the resources within the subnet. See aka.ms/deletesubnet.\" Details=[]\n",
			},
			want: []TFApplyResult{
				{Destroying: 1, Object: "module.aks1.azurerm_subnet.this", Action: "destroying", Elapsed: ""},
				{Destroying: 1, Object: "module.aks1.azurerm_subnet.this", Action: "destruction", Elapsed: "0s",
					Text: "module.aks1.azurerm_subnet.this: Destruction complete after 0s",
				},
				{Destroying: 1,
					Errors: []string{"Error deleting Subnet \"intlb\" (Virtual Network \"yyy-vnet\" / Resource Group \"xxx-rg\"): network.SubnetsClient#Delete: Failure sending request: StatusCode=400 -- Original Error: Code=\"InUseSubnetCannotBeDeleted\" Message=\"Subnet intlb is in use by /subscriptions/ea365/resourceGroups/mc_xxx-rg_x_westeurope/providers/Microsoft.Network/loadBalancers/kubernetes-internal/frontendIPConfigurations/a57-intlb and cannot be deleted. In order to delete the subnet, delete all the resources within the subnet. See aka.ms/deletesubnet.\" Details=[]"},
					Text:   "Error: Error deleting Subnet \"intlb\" (Virtual Network \"yyy-vnet\" / Resource Group \"xxx-rg\"): network.SubnetsClient#Delete: Failure sending request: StatusCode=400 -- Original Error: Code=\"InUseSubnetCannotBeDeleted\" Message=\"Subnet intlb is in use by /subscriptions/ea365/resourceGroups/mc_xxx-rg_x_westeurope/providers/Microsoft.Network/loadBalancers/kubernetes-internal/frontendIPConfigurations/a57-intlb and cannot be deleted. In order to delete the subnet, delete all the resources within the subnet. See aka.ms/deletesubnet.\" Details=[]",
				},
			},
		},
		{
			it: "must parse a successful apply",
			in: []string{
				"azurerm_route_table.env: Modifying... [id=/subscriptions/ea363b8e/resourceGroups/xxx-rg/providers/Microsoft.Network/routeTables/yyy-routetable]\n",
				"module.aks1.azurerm_kubernetes_cluster.this: Destroying... [id=/subscriptions/ea363b8e/resourcegroups/xxx-rg/providers/Microsoft.ContainerService/managedClusters/yyy-cpe]\n",
				"azurerm_route_table.env: Modifications complete after 1s [id=/subscriptions/ea363b8e/resourceGroups/xxx-rg/providers/Microsoft.Network/routeTables/yyy-routetable]\n",
				"module.aks1.azurerm_subnet.this: Modifying... [id=/subscriptions/ea363b8e/resourceGroups/xxx-rg/providers/Microsoft.Network/virtualNetworks/yyy-vnet/subnets/cpe]\n",
				"module.aks1.azurerm_kubernetes_cluster.this: Still destroying... [id=/subscriptions/ea363b8e-...inerService/managedClusters/yyy-cpe, 10s elapsed]\n",
				"module.aks1.azurerm_subnet.this: Still modifying... [id=/subscriptions/ea363b8e-ceb3-40ab-9662-...irtualNetworks/yyy-vnet/subnets/cpe, 10s elapsed]\n",
				"module.aks1.azurerm_kubernetes_cluster.this: Still destroying... [id=/subscriptions/ea363b8e-...inerService/managedClusters/yyy-cpe, 20s elapsed]\n",
				"module.aks1.azurerm_subnet.this: Still modifying... [id=/subscriptions/ea363b8e-...irtualNetworks/yyy-vnet/subnets/cpe, 20s elapsed]\n",
				"module.aks1.azurerm_kubernetes_cluster.this: Still destroying... [id=/subscriptions/ea363b8e-...inerService/managedClusters/yyy-cpe, 30s elapsed]\n",
				"module.aks1.azurerm_subnet.this: Still modifying... [id=/subscriptions/ea363b8e-...irtualNetworks/yyy-vnet/subnets/cpe, 30s elapsed]\n",
				"module.aks1.azurerm_subnet.this: Modifications complete after 32s [id=/subscriptions/ea363b8e/resourceGroups/xxx-rg/providers/Microsoft.Network/virtualNetworks/yyy-vnet/subnets/cpe]\n",
				"module.aks1.azurerm_kubernetes_cluster.this: Still destroying... [id=/subscriptions/ea363b8e-...inerService/managedClusters/yyy-cpe, 40s elapsed]\n",
				"module.aks1.azurerm_kubernetes_cluster.this: Still destroying... [id=/subscriptions/ea363b8e-...inerService/managedClusters/yyy-cpe, 50s elapsed]\n",
				"module.aks1.azurerm_kubernetes_cluster.this: Still destroying... [id=/subscriptions/ea363b8e-...inerService/managedClusters/yyy-cpe, 1m0s elapsed]\n",
				"module.aks1.azurerm_kubernetes_cluster.this: Destruction complete after 1m8s\n",
				"module.aks1.azurerm_kubernetes_cluster.this: Creating...\n",
				"module.aks1.azurerm_kubernetes_cluster.this: Still creating... [10s elapsed]\n",
				"module.aks1.azurerm_kubernetes_cluster.this: Still creating... [20s elapsed]\n",
				"module.aks1.azurerm_kubernetes_cluster.this: Still creating... [30s elapsed]\n",
				"module.aks1.azurerm_kubernetes_cluster.this: Creation complete after 6m22s [id=/subscriptions/ea363b8e/resourcegroups/xxx-rg/providers/Microsoft.ContainerService/managedClusters/yyy-cpe]\n",
				"\nApply complete! Resources: 1 added, 2 changed, 1 destroyed.\n",
				"The state of your infrastructure has been saved to the path\nbelow. This state is required to modify and destroy your\ninfrastructure, so keep it safe. To inspect the complete state\nuse the `terraform show` command.\n				\n",
				"State path: terraform.tfstate\n"},
			want: []TFApplyResult{
				{Modifying: 1, Object: "azurerm_route_table.env", Action: "modifying", Elapsed: "",
					Text: "",
				},
				{Modifying: 1, Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "destroying", Elapsed: "",
					Text: "",
				},
				{Modifying: 1, Destroying: 1, Object: "azurerm_route_table.env", Action: "modifications", Elapsed: "1s",
					Text: "azurerm_route_table.env: Modifications complete after 1s [id=/subscriptions/ea363b8e/resourceGroups/xxx-rg/providers/Microsoft.Network/routeTables/yyy-routetable]",
				},
				{Modifying: 2, Destroying: 1, Object: "module.aks1.azurerm_subnet.this", Action: "modifying", Elapsed: "",
					Text: "",
				},
				{Modifying: 2, Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "destroying", Elapsed: "10s",
					Text: "module.aks1.azurerm_kubernetes_cluster.this: Still destroying... [id=/subscriptions/ea363b8e-...inerService/managedClusters/yyy-cpe, 10s elapsed]",
				},
				{Modifying: 2, Destroying: 1, Object: "module.aks1.azurerm_subnet.this", Action: "modifying", Elapsed: "10s",
					Text: "module.aks1.azurerm_subnet.this: Still modifying... [id=/subscriptions/ea363b8e-ceb3-40ab-9662-...irtualNetworks/yyy-vnet/subnets/cpe, 10s elapsed]",
				},
				{Modifying: 2, Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "destroying", Elapsed: "20s",
					Text: "module.aks1.azurerm_kubernetes_cluster.this: Still destroying... [id=/subscriptions/ea363b8e-...inerService/managedClusters/yyy-cpe, 20s elapsed]",
				},
				{Modifying: 2, Destroying: 1, Object: "module.aks1.azurerm_subnet.this", Action: "modifying", Elapsed: "20s",
					Text: "module.aks1.azurerm_subnet.this: Still modifying... [id=/subscriptions/ea363b8e-...irtualNetworks/yyy-vnet/subnets/cpe, 20s elapsed]",
				},
				{Modifying: 2, Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "destroying", Elapsed: "30s",
					Text: "module.aks1.azurerm_kubernetes_cluster.this: Still destroying... [id=/subscriptions/ea363b8e-...inerService/managedClusters/yyy-cpe, 30s elapsed]",
				},
				{Modifying: 2, Destroying: 1, Object: "module.aks1.azurerm_subnet.this", Action: "modifying", Elapsed: "30s",
					Text: "module.aks1.azurerm_subnet.this: Still modifying... [id=/subscriptions/ea363b8e-...irtualNetworks/yyy-vnet/subnets/cpe, 30s elapsed]",
				},
				{Modifying: 2, Destroying: 1, Object: "module.aks1.azurerm_subnet.this", Action: "modifications", Elapsed: "32s",
					Text: "module.aks1.azurerm_subnet.this: Modifications complete after 32s [id=/subscriptions/ea363b8e/resourceGroups/xxx-rg/providers/Microsoft.Network/virtualNetworks/yyy-vnet/subnets/cpe]",
				},
				{Modifying: 2, Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "destroying", Elapsed: "40s",
					Text: "module.aks1.azurerm_kubernetes_cluster.this: Still destroying... [id=/subscriptions/ea363b8e-...inerService/managedClusters/yyy-cpe, 40s elapsed]",
				},
				{Modifying: 2, Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "destroying", Elapsed: "50s",
					Text: "module.aks1.azurerm_kubernetes_cluster.this: Still destroying... [id=/subscriptions/ea363b8e-...inerService/managedClusters/yyy-cpe, 50s elapsed]",
				},
				{Modifying: 2, Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "destroying", Elapsed: "1m0s",
					Text: "module.aks1.azurerm_kubernetes_cluster.this: Still destroying... [id=/subscriptions/ea363b8e-...inerService/managedClusters/yyy-cpe, 1m0s elapsed]",
				},
				{Modifying: 2, Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "destruction", Elapsed: "1m8s",
					Text: "module.aks1.azurerm_kubernetes_cluster.this: Destruction complete after 1m8s",
				},
				{Creating: 1, Modifying: 2, Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "creating", Elapsed: "",
					Text: "",
				},
				{Creating: 1, Modifying: 2, Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "creating", Elapsed: "[10s",
					Text: "module.aks1.azurerm_kubernetes_cluster.this: Still creating... [10s elapsed]",
				},
				{Creating: 1, Modifying: 2, Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "creating", Elapsed: "[20s",
					Text: "module.aks1.azurerm_kubernetes_cluster.this: Still creating... [20s elapsed]",
				},
				{Creating: 1, Modifying: 2, Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "creating", Elapsed: "[30s",
					Text: "module.aks1.azurerm_kubernetes_cluster.this: Still creating... [30s elapsed]",
				},
				{Creating: 1, Modifying: 2, Destroying: 1, Object: "module.aks1.azurerm_kubernetes_cluster.this", Action: "creation", Elapsed: "6m22s",
					Text: "module.aks1.azurerm_kubernetes_cluster.this: Creation complete after 6m22s [id=/subscriptions/ea363b8e/resourcegroups/xxx-rg/providers/Microsoft.ContainerService/managedClusters/yyy-cpe]",
				},
				{Creating: 1, Modifying: 2, Destroying: 1, TotalAdded: 1, TotalChanged: 2, TotalDestroyed: 1, Object: "", Action: "", Elapsed: "",
					Text: "Apply complete! Resources: 1 added, 2 changed, 1 destroyed.",
				},
			},
		},
		{
			it: "must parse a successful destroy",
			in: []string{
				"data.azurerm_resource_group.env: Refreshing state...\n",
				"azurerm_virtual_network.env: Refreshing state... [id=/subscriptions/ea365/resourceGroups/xxx-rg/providers/Microsoft.Network/virtualNetworks/yyy-vnet]\n",
				"azurerm_subnet.intlb: Refreshing state... [id=/subscriptions/ea365/resourceGroups/xxx-rg/providers/Microsoft.Network/virtualNetworks/yyy-vnet/subnets/intlb]\n",
				"azurerm_subnet.intlb: Destroying... [id=/subscriptions/ea365/resourceGroups/xxx-rg/providers/Microsoft.Network/virtualNetworks/yyy-vnet/subnets/intlb]\n",
				"azurerm_subnet.intlb: Destruction complete after 0s\n",
				"azurerm_virtual_network.env: Destroying... [id=/subscriptions/ea365/resourceGroups/xxx-rg/providers/Microsoft.Network/virtualNetworks/yyy-vnet]\n",
				"azurerm_virtual_network.env: Still destroying... [id=/subscriptions/ea365-...ft.Network/virtualNetworks/yyy-vnet, 10s elapsed]\n",
				"azurerm_virtual_network.env: Destruction complete after 11s\n",
				"\nDestroy complete! Resources: 2 destroyed.\n",
			},
			want: []TFApplyResult{
				{Destroying: 1, Object: "azurerm_subnet.intlb", Action: "destroying", Elapsed: ""},
				{Destroying: 1, Object: "azurerm_subnet.intlb", Action: "destruction", Elapsed: "0s",
					Text: "azurerm_subnet.intlb: Destruction complete after 0s",
				},
				{Destroying: 2, Object: "azurerm_virtual_network.env", Action: "destroying", Elapsed: ""},
				{Destroying: 2, Object: "azurerm_virtual_network.env", Action: "destroying", Elapsed: "10s",
					Text: "azurerm_virtual_network.env: Still destroying... [id=/subscriptions/ea365-...ft.Network/virtualNetworks/yyy-vnet, 10s elapsed]",
				},
				{Destroying: 2, Object: "azurerm_virtual_network.env", Action: "destruction", Elapsed: "11s",
					Text: "azurerm_virtual_network.env: Destruction complete after 11s",
				},
				{Destroying: 2, TotalDestroyed: 2,
					Text: "Destroy complete! Resources: 2 destroyed.",
				},
			},
		},
		{
			it:   "must handle empty input",
			in:   []string{},
			want: []TFApplyResult{},
		},
	}

	tf := &Terraform{}

	log := testr.New(t)
	for _, tst := range tsts {
		t.Run(tst.it, func(t *testing.T) {

			rd, wr := io.Pipe()

			// start parser
			ch := tf.parseAsyncApplyResponse(log, rd)

			// send input
			go func() {
				var err error
				for _, s := range tst.in {
					_, err = wr.Write([]byte(s))
					assert.NoError(t, err)
				}
				err = wr.Close()
				assert.NoError(t, err)
			}()

			// read output
			rs := []TFApplyResult{}
			for r := range ch {
				rs = append(rs, r)
			}

			assert.Equal(t, tst.want, rs)
		})
	}
}

// NewExitError returns an ExitError with exit code.
func newExitError(code int) error {
	cmd := exec.Command("exit", strconv.Itoa(code))
	return cmd.Run()
}
