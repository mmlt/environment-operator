package azure

import (
	"github.com/mmlt/environment-operator/pkg/util/exe"
	"strings"
)

// KeyvaultSecret returns the value of 'name' secret in 'vaultName' KeyVault.
func (c *AZ) KeyvaultSecret(name, vaultName string) (string, error) {
	args := []string{"keyvault", "secret", "show", "--name", name, "--vault-name", vaultName, "--query", "value", "-o", "tsv"}
	o, _, err := exe.Run(c.Log, nil, "", "az", args...)
	if err != nil {
		return "", err
	}

	return strings.TrimSuffix(o, "\n"), nil
}
