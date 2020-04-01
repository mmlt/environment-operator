package _source

// FetchFake pretends to update Content from a source while in reality it places some test data in the local file path.
func (c *Content) FetchFake() error {
	err := c.assertTempDir()
	if err != nil {
		return err
	}

	err = c.writeFileS("main.tf.tmplt", `
module "aks1" {
  source = "./modules/aks"

  name                = var.clusters[0].name                //"cpe"
  subnet_num          = var.clusters[0].subnet_num          //1
  k8s_version         = var.clusters[0].version             //"1.14.8"
  defaultpool_scale   = var.clusters[0].defaultpool_scale   //2
  defaultpool_vm_size = var.clusters[0].defaultpool_vm_size //"Standard_DS2_v2"
  node_pools          = var.clusters[0].node_pools

  resource_group = data.azurerm_resource_group.env
  env_name       = var.env_name
  vnet           = azurerm_virtual_network.env
  subnet_newbits = var.subnet_newbits
  route_table_id = azurerm_route_table.env.id
  vnet_sp_oauth  = local.vnet_sp_oauth
  aad            = var.aad
  tags           = local.tags
}

/*
module "aks2" {
  source = "./modules/aks"

  name = "test2"
  subnet_num = 2
  k8s_version = "1.14.8"
  defaultpool_scale = 2
  defaultpool_vm_size = "Standard_DS2_v2"

  resource_group = data.azurerm_resource_group.env
  env_name = var.env_name
  vnet = azurerm_virtual_network.env
  subnet_newbits = var.subnet_newbits
  vnet_sp_oauth = local.vnet_sp_oauth
  aad = var.aad
  tags = local.tags
}*/
`)
	if err != nil {
		return err
	}

	return nil
}

// WriteFileS provides same functionality as WriteFile but takes string data.
func (c *Content) writeFileS(name, data string) error {
	return c.WriteFile(name, []byte(data))
}
