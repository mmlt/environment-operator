# AKS cluster with Advanced networking and AAD integration.
# Each AKS cluster get its own subnet in the vnet.
# https://www.terraform.io/docs/providers/azurerm/r/kubernetes_cluster.html

resource "azurerm_subnet" "this" {
  name                 = var.name
  resource_group_name  = var.resource_group.name
  address_prefix       = cidrsubnet(var.vnet.address_space[0], var.subnet_newbits, var.subnet_num)
  virtual_network_name = var.vnet.name
  // TODO remove when 2.0 arrives, see https://github.com/terraform-providers/terraform-provider-azurerm/issues/2358
  route_table_id = var.route_table_id
}
resource "azurerm_subnet_route_table_association" "this" {
  subnet_id      = azurerm_subnet.this.id
  route_table_id = var.route_table_id
}

resource "azurerm_kubernetes_cluster" "this" {
  name                = "${var.env_name}-${var.name}"
  resource_group_name = var.resource_group.name
  location            = var.resource_group.location
  dns_prefix          = "${var.env_name}-${var.name}"

  //TODO see Task 4305: Pod Security Policy 
  //enable_pod_security_policy = true
  kubernetes_version = var.k8s_version

  /*TODO remove  linux_profile {
    admin_username = "user1"
    ssh_key {
      key_data = file(var.public_ssh_key_path)
    }
  }*/

  default_node_pool {
    name    = "default"
    vm_size = var.defaultpool_vm_size
    #max_pods  =
    type = "VirtualMachineScaleSets"
    // Autoscaling
    enable_auto_scaling = false
    node_count          = var.defaultpool_scale
    // TODO Enable node autoscaler
    #enable_auto_scaling = true
    #max_count = 10
    #min_count = 1
    #node_count = 2

    #availability_zones = [1,2]
    #os_disk_size_gb
    #node_taints

    vnet_subnet_id = azurerm_subnet.this.id
  }


  service_principal {
    client_id     = var.vnet_sp_oauth.client_id
    client_secret = var.vnet_sp_oauth.client_secret
  }

  network_profile {
    load_balancer_sku = "standard"
    network_plugin    = "azure"
    //TODO see Task 4155: Network policy
    //network_policy     = "calico"
  }

  role_based_access_control {
    enabled = true

    azure_active_directory {
      tenant_id         = var.aad.tenant_id
      server_app_id     = var.aad.server_app_id
      server_app_secret = var.aad.server_app_secret
      client_app_id     = var.aad.client_app_id
    }
  }

  addon_profile {
    kube_dashboard {
      enabled = false
    }
  }

  lifecycle {
    ignore_changes = [
      default_node_pool[0].node_count
    ]
  }

  tags = var.tags
}

resource "azurerm_kubernetes_cluster_node_pool" "this" {
  for_each              = var.node_pools
  kubernetes_cluster_id = azurerm_kubernetes_cluster.this.id
  name                  = each.key
  vm_size               = each.value.vm_size
  node_count            = each.value.scale
}