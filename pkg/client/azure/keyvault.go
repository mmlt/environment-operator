package azure

import (
	"strings"
)

// KeyvaultSecret returns the value of 'name' secret in 'vaultName' KeyVault.
func (c *AZ) KeyvaultSecret(name, vaultName string) (string, error) {
	args := []string{"keyvault", "secret", "show", "--name", name, "--vault-name", vaultName, "--query", "value", "-o", "tsv"}
	o, err := runAZ(c.Log, nil, "", args...)
	if err != nil {
		return "", err
	}

	return strings.TrimSuffix(o, "\n"), nil
}
