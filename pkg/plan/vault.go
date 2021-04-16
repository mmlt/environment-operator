package plan

import (
	"fmt"
	"github.com/hashicorp/go-multierror"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/cloud"
	"strings"
)

// VaultInfraValues replaces references to a vault value with the actual value.
// A value is considered a reference when it uses the form "vault secretname secretfield"
func vaultInfraValues(infra v1.InfraSpec, c cloud.Cloud) (v1.InfraSpec, error) {
	var err error

	err = vaultValue(&infra.Source.Ref, c, "infra.source.ref", err)

	err = vaultValue(&infra.State.Access, c, "access", err)
	err = vaultValue(&infra.AAD.TenantID, c, "tenantID", err)
	err = vaultValue(&infra.AAD.ClientAppID, c, "clientAppID", err)
	err = vaultValue(&infra.AAD.ServerAppID, c, "serverAppID", err)
	err = vaultValue(&infra.AAD.ServerAppSecret, c, "serverAppSecret", err)

	return infra, err
}

// VaultClusterValues replaces references to a vault value with the actual value.
// A value is considered a reference when it uses the form "vault secretname secretfield"
func vaultClusterValues(clusters []v1.ClusterSpec, c cloud.Cloud) ([]v1.ClusterSpec, error) {
	var err error

	for _, cluster := range clusters {
		err = vaultValue(&cluster.Addons.Source.Ref, c, "addons.source.ref", err)
	}

	return clusters, err
}

// VaultValue changes an s with "vault name field" to the referenced value.
func vaultValue(s *string, c cloud.Cloud, msg string, errs error) error {
	if !strings.HasPrefix(*s, "vault ") {
		return nil
	}

	ss := strings.Fields(*s)
	var n, f string
	switch len(ss) {
	case 2:
		n = ss[1]
	case 3:
		n = ss[1]
		f = ss[2]
	default:
		return multierror.Append(errs, fmt.Errorf("field %s: vault reference wrong", msg))
	}

	v, err := c.VaultGet(n, f)
	if err != nil {
		return multierror.Append(errs, err)
	}

	*s = v

	return errs
}
