package azure

import (
	"encoding/json"
)

// LoginSP performs an 'az login' by ServicePrincipal.
func (c *AZ) LoginSP(user, password, tenant string) error {
	args := []string{"login", "--service-principal", "-u", user, "-p", password, "--tenant", tenant}
	_, err := runAZ(c.Log, nil, "", args...)
	if err != nil {
		return err
	}

	return nil
}

// Logout performs an 'az logout'.
func (c *AZ) Logout() error {
	args := []string{"logout"}
	_, err := runAZ(c.Log, nil, "", args...)
	if err != nil {
		return err
	}

	return nil
}

// AccountStatus returns the account status of the already logged in account.
// An error is returned when no account is logged in.
func (c *AZ) AccountStatus() ([]AccountStatus, error) {
	args := []string{"account", "show"}
	o, err := runAZ(c.Log, nil, "", args...)
	if err != nil {
		return nil, err
	}

	var r []AccountStatus
	err = json.Unmarshal([]byte(o), &r)
	if err != nil {
		return nil, err
	}

	return r, nil
}

// AccountStatus is a subset of az account status values.
type AccountStatus struct {
	// Name of the subscription.
	Name string `json:"name"`
	// ID of the subscription.
	Id string `json:"id"`
	// IsDefault is true for the default account.
	IsDefault bool `json:"isDefault"`
	// State of the account
	AccountState AccountState `json:"state"`
	// User that is logged in.
	User struct {
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"user"`
}

type AccountState string

const (
	// Creating means ContainerService resource is being created.
	Enabled  AccountState = "Enabled"
	Disabled AccountState = "Disabled"
)
