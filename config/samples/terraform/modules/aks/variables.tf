variable "resource_group" {
  description = "The Resource Group that is used for creating resources."
}

variable "env_name" {
  description = <<-ETX
    Environment name is used a prefix for resources and as part of the domain name of services running in the clusters.
    For example for name=cpe, env_name=k8ss the domain becomes: app.namespace.cpe.k8ss.example.com
    Typically "k8s{data.azurerm_resource_group.env.tags[EnvironmentType]}"
  ETX
}

variable "vnet" {
  description = "VNet in which AKS clusters get a subnet."
}

variable "route_table_id" {
  description = "Routing for all subnets containing Pods."
}

variable "vnet_sp_oauth" {
  description = "Service Principal OAuth credentials that AKS uses to access VNet."
  type = object({
    client_id     = string
    client_secret = string
  })
}

variable "aad" {
  description = "AzureAD app's used by clusters for RBAC of users."
  type = object({
    tenant_id         = string
    server_app_id     = string
    server_app_secret = string
    client_app_id     = string
  })
}

variable "name" {
  description = "Name of cluster, also used in domain name; app.namespace.<name>.env.subdomain."
  type        = string
}

variable "subnet_num" {
  description = "Subnet index of vnet that is used by this cluster. Valid from 1 to (2^subnet_newbits)-1"
  type        = number
}

variable "subnet_newbits" {
  description = <<-ETX
    Subnet newbits is the number of bits to add to the VNet address mask to produce the subnet mask.
    For example given a /16 VNet and subnet_newbits=4 would result in /20 subnets.
    Note: this values must be the same for all subnets in the vnet.
  ETX
  type        = number
  default     = 4
}

variable "k8s_version" {
  description = "Kubernetes version."
  type        = string
}

variable "defaultpool_scale" {
  description = "Number of worker nodes in default pool."
  type        = number
}

variable "defaultpool_vm_size" {
  description = "VM type of worker nodes in default pool."
  type        = string
}

variable "node_pools" {
  description = "List of additional node pools."
  type = map(object({
    # key is used as name
    scale   = number
    vm_size = string
  }))
}

variable "tags" {
  description = "Azure resource tags to comply with organisation policy."
}