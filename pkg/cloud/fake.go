package cloud

type Fake struct {
}

var _ Cloud = &Fake{}

func (f Fake) Login() (*ServicePrincipal, error) {
	sp := &ServicePrincipal{
		ClientID:     "clientid",
		ClientSecret: "clientsecret",
		Tenant:       "tenant",
	}
	return sp, nil
}

func (f Fake) VaultGet(name, field string) (string, error) {
	return "vaultval", nil
}
