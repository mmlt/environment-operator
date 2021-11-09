package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Cluster contains the data to access a k8s cluster in an environment.
type Cluster struct {
	// Environment in which the cluster is located (K8sEnvironment eu41d)
	Environment string
	// Name of the cluster (K8sCluster one)
	Name string
	// Domain is the DNS domain (K8sDomain example inf.iitech.dk)
	Domain string
	// Provider Cloud provider (K8sProvider aks)
	Provider string
	// Config contains a kubeconfig to access the cluster.
	Config []byte
}

// Equal returns true if lhs and rhs are equal.
func (lhs Cluster) Equal(rhs Cluster) bool {
	if lhs.Environment != rhs.Environment {
		return false
	}
	if lhs.Name != rhs.Name {
		return false
	}
	if lhs.Domain != rhs.Domain {
		return false
	}
	if lhs.Provider != rhs.Provider {
		return false
	}

	// serializing kubeconfig into Config is expected to be stable (encoding/json is)
	return bytes.Equal(lhs.Config, rhs.Config)
}

// SecretToCluster returns the cluster data contained by secret.
func SecretToCluster(secret corev1.Secret) (*Cluster, error) {
	c := &Cluster{}
	err := json.Unmarshal(secret.Data["cluster"], c)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// SecretFromCluster returns a Secret containing cluster data.
func SecretFromCluster(cluster Cluster) (*corev1.Secret, error) {
	cjson, err := json.Marshal(cluster)
	if err != nil {
		return nil, err
	}

	secret := &corev1.Secret{
		Data: map[string][]byte{
			"cluster": cjson,
		},
	}

	return secret, nil
}

// Client is used to access Secrets containing cluster data.
type Client struct {
	Client client.Client

	// Labels is the set of labels that is required.
	Labels labels.Set
}

// List reads cluster Secrets and returns a []Cluster.
func (cl Client) List(ctx context.Context, namespace string) ([]Cluster, error) {
	secrets := &corev1.SecretList{}
	err := cl.Client.List(ctx, secrets, &client.ListOptions{
		Namespace:     namespace,
		LabelSelector: cl.Labels.AsSelector(),
	})
	if err != nil {
		return nil, err
	}

	var result []Cluster
	for _, secret := range secrets.Items {
		cluster, err := SecretToCluster(secret)
		if err != nil {
			return nil, err
		}

		result = append(result, *cluster)
	}

	return result, nil
}

// Create creates a Secret for each item in clusters.
func (cl Client) Create(ctx context.Context, namespace string, clusters []Cluster) error {
	for _, cluster := range clusters {
		sec, err := SecretFromCluster(cluster)
		if err != nil {
			return err
		}

		sec.SetNamespace(namespace)
		sec.SetName(cluster.Environment + "-" + cluster.Name)
		sec.SetLabels(cl.Labels)

		err = cl.Client.Create(ctx, sec, &client.CreateOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

// Update updates Secrets with clusters.
func (cl Client) Update(ctx context.Context, namespace string, clusters []Cluster) error {
	for _, cluster := range clusters {
		sec, err := SecretFromCluster(cluster)
		if err != nil {
			return err
		}

		sec.SetNamespace(namespace)
		sec.SetName(cluster.Environment + "-" + cluster.Name)
		sec.SetLabels(cl.Labels)

		err = cl.Client.Update(ctx, sec, &client.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

// Delete deletes Secrets corresponding to clusters.
func (cl Client) Delete(ctx context.Context, namespace string, clusters []Cluster) error {
	for _, cluster := range clusters {
		sec := &corev1.Secret{}
		sec.SetNamespace(namespace)
		sec.SetName(cluster.Environment + "-" + cluster.Name)
		sec.SetLabels(cl.Labels)

		err := cl.Client.Delete(ctx, sec, &client.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

// Diff compares current with desired state and returns clusters to create, update, delete.
func Diff(current, desired []Cluster) (create, update, delete []Cluster) {
	// index states
	ci := map[string]Cluster{}
	for _, c := range current {
		ci[c.Name] = c
	}

	di := map[string]Cluster{}
	for _, d := range desired {
		di[d.Name] = d
	}

	// delete
	for _, c := range current {
		if _, ok := di[c.Name]; !ok {
			delete = append(delete, c)
		}
	}

	// update or create
	for _, d := range desired {
		if c, ok := ci[d.Name]; ok {
			if !c.Equal(d) {
				update = append(update, d)
			}
		} else {
			create = append(create, d)
		}
	}

	return
}
