// Package cloud provides cloud generic operations.
package cloud

type Cloud interface {
	// Login perform cloud provider login.
	Login() (*ServicePrincipal, error)
	// VaultGet reads a secret from a vault.
	VaultGet(name, field string) (string, error)
}

type ServicePrincipal struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Tenant       string `json:"tenant"`
}
