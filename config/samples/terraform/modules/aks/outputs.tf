output "id" {
  value = azurerm_kubernetes_cluster.this.id
}

output "host" {
  value = azurerm_kubernetes_cluster.this.kube_config[0].host
}

output "cluster_ca_certificate" {
  value = azurerm_kubernetes_cluster.this.kube_config[0].cluster_ca_certificate
}

output "client_certificate" {
  value = azurerm_kubernetes_cluster.this.kube_config[0].client_certificate
}

output "client_key" {
  value = azurerm_kubernetes_cluster.this.kube_config[0].client_key
}

// kube_config isn't needed as all important data is in the above fields.
//output "kube_config" {
//  value = azurerm_kubernetes_cluster.this.kube_config_raw
//}