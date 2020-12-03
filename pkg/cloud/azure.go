package cloud

import (
	"encoding/json"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/mmlt/environment-operator/pkg/client/azure"
	gocache "github.com/patrickmn/go-cache"
	"io/ioutil"
	"time"
)

type Azure struct {
	Client azure.AZer
	// CredentialsFile is the path to a JSON formatted file containing client_id, client_secret and tenant of a
	// ServicePrincipal that is allowed to access the MasterKeyVault and AzureRM.
	CredentialsFile string
	// Vault is the name of the KeyVault to access.
	Vault string

	Log logr.Logger

	// Secret contains the envop SP after first successful login.
	secret *ServicePrincipal

	// Cache contains response values to support rate-limiting.
	cache *gocache.Cache
}

// Login performs a cloud provider login (a prerequisite for most cli commands)
func (a *Azure) Login() (*ServicePrincipal, error) {
	if a.secret != nil {
		return a.secret, nil
	}

	// get login secret
	b, err := ioutil.ReadFile(a.CredentialsFile)
	if err != nil {
		return nil, err
	}
	var sp ServicePrincipal
	err = json.Unmarshal(b, &sp)
	if err != nil {
		return nil, err
	}
	if len(sp.ClientID) == 0 || len(sp.ClientSecret) == 0 || len(sp.Tenant) == 0 {
		return nil, fmt.Errorf("login secret: client_id, cliebnt_secret or tenant field not set")
	}

	// login
	err = a.Client.LoginSP(sp.ClientID, sp.ClientSecret, sp.Tenant)
	if err != nil {
		return nil, err
	}

	a.secret = &sp

	a.Log.Info("Logged in")

	return a.secret, nil
}

// VaultGet reads a secret from a vault.
// Vault access is rate limited to once per 5m.
func (a *Azure) VaultGet(name, field string) (string, error) {
	_, err := a.Login()
	if err != nil {
		return "", err
	}

	if a.cache == nil {
		a.cache = gocache.New(5*time.Minute, 10*time.Minute)
	}

	var v string
	x, ok := a.cache.Get(name)
	if ok {
		v = x.(string)
	} else {
		v, err = a.Client.KeyvaultSecret(name, a.Vault)
		if err != nil {
			return "", err
		}
		a.cache.SetDefault(name, v)
	}

	if field == "" || field == "." {
		return v, nil
	}

	m := map[string]string{}
	err = json.Unmarshal([]byte(v), &m)
	if err != nil {
		return "", err
	}

	if v := m[field]; v != "" {
		return v, nil
	}

	err = fmt.Errorf("no field '%s' in secret %s", field, name)
	return "", err
}
