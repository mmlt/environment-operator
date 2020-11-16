package cloud

import (
	"encoding/json"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/mmlt/environment-operator/pkg/client/azure"
	"io/ioutil"
)

type Azure struct {
	Client azure.AZer
	// CredentialsFile contains the client_id, client_secret and tenant of a ServicePrincipal that is allowed to access
	// the MasterKeyVault and AzureRM.
	CredentialsFile string

	// TFStateSecretName is the name of a secret which value allows access to the the blob storage containing Terraform state.
	TFStateSecretName string
	// TFStateSecretVault is the name of the KeyVault that contains TFStateSecretName.
	TFStateSecretVault string

	Log logr.Logger

	// LoggedIn is true after first successful login.
	loggedIn bool

	// Environ is the collection of cloud specific environment variables that should me made available to steps.
	environ map[string]string
}

func (a *Azure) Login() (map[string]string, error) {
	if a.loggedIn {
		return a.environ, nil
	}

	// get login secret
	b, err := ioutil.ReadFile(a.CredentialsFile)
	if err != nil {
		return nil, err
	}
	secret := struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		Tenant       string `json:"tenant"`
	}{}
	err = json.Unmarshal(b, &secret)
	if err != nil {
		return nil, err
	}
	if len(secret.ClientID) == 0 || len(secret.ClientSecret) == 0 || len(secret.Tenant) == 0 {
		return nil, fmt.Errorf("login secret: user, password or tenant field not set")
	}

	// login
	err = a.Client.LoginSP(secret.ClientID, secret.ClientSecret, secret.Tenant)
	if err != nil {
		return nil, err
	}

	// get environment
	if a.environ == nil {
		a.environ = make(map[string]string)
	}
	a.environ["ARM_CLIENT_ID"] = secret.ClientID
	a.environ["ARM_CLIENT_SECRET"] = secret.ClientSecret
	a.environ["ARM_TENANT_ID"] = secret.Tenant

	v, err := a.Client.KeyvaultSecret(a.TFStateSecretName, a.TFStateSecretVault)
	if err != nil {
		return nil, err
	}
	a.environ["ARM_ACCESS_KEY"] = v

	a.loggedIn = true

	a.Log.Info("Login", "loggedin", a.loggedIn)

	return a.environ, nil
}
