// Package plan contains the work to do and the current status.
package plan

// Package plan contains the work to do and the current status.
// It is mainly a wrapper around the CR.

/*
- plan.clusters = list of clusters from spec.config + spec.clusters (the latter overriding values of the previous)
  - no overrides in provision source allowed (because we have a single terraform config for all clusters)
  - override of addons source `branch:` allowed
- for all sources readSource
- remove all clusters of which the deployment SHA's match their source spec SHA
- resolve internal jq path values (resolve secrets)
*/

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"github.com/go-logr/logr"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/repo/fs"
	"github.com/mmlt/environment-operator/pkg/repo/git"
	"io/ioutil"
	"os"
)

type Plan struct {
	//
	CR *v1.Environment

	//
	//Sources

	//
	//Status

	log logr.Logger
}

// New returns a plan for a given environment.
func New(cr *v1.Environment, log logr.Logger) *Plan {
	return &Plan{
		CR:  cr,
		log: log,
	}
}

// Values are derived from the (dynamic) values provided by the CR.
type Values struct {
	// Default
	Defaults map[string]string
	// Clusters contains default values merged with cluster values.
	// The only thing we know for sure is that ["name"] contains the cluster name.
	Clusters []map[string]string
}

// InfrastructureValues returns 'default' and 'cluster' key-values.
func (p *Plan) InfrastructureValues() *Values {
	// TODO consider to use yamlx.Merge() instead
	var values []map[string]string
	for _, c := range p.CR.Spec.Clusters {
		o := map[string]string{}
		for k, v := range p.CR.Spec.Defaults.Infrastructure.Values {
			o[k] = v
		}
		for k, v := range c.Infrastructure.Values {
			o[k] = v
		}
		o["name"] = c.Name
		values = append(values, o)
	}

	return &Values{
		Defaults: p.CR.Spec.Defaults.Infrastructure.Values,
		Clusters: values,
	}
}

// InfrastructureValuesJSON returns InfrastructureValues() as JSON formatted text.
/*func (p *Plan) InfrastructureValuesJSON() ([]byte, error) {
	v := p.InfrastructureValues()
	return json.Marshal(v)
}*/

// Source is a local copy of a repository of files.
type Source interface {
	Name() string
	Update() error
	RepoDir() string
	Remove() error
}

// InfrastructureSource returns the infrastructure source.
func (p *Plan) InfrastructureSource() (Source, error) {
	//TODO lazy init (cache in plan)?
	spec := p.CR.Spec.Defaults.Infrastructure.Source
	switch spec.Type {
	case v1.SourceTypeGIT:
		return git.New(spec.URL, spec.Ref, spec.Token, os.TempDir(), p.log)
	case v1.SourceTypeLocal:
		return fs.New(spec.URL, os.TempDir(), p.log)
	}

	return nil, fmt.Errorf("%s: unknown source type: %v", spec.URL, spec.Type)
}

// GetTFState reads the Terraform state from CR.
func (p *Plan) GetTFState() ([]byte, error) {
	// base64
	b, err := base64.StdEncoding.DecodeString(p.CR.Status.TFState)
	if err != nil {
		return nil, err
	}

	if len(b) == 0 {
		return []byte{}, nil
	}

	// gzip
	gz, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	return ioutil.ReadAll(gz)
}

// PutTFState write the Terraform state to CR.
func (p *Plan) PutTFState(state []byte) error {
	// gzip
	var bb bytes.Buffer
	gz := gzip.NewWriter(&bb)
	_, err := gz.Write(state)
	if err != nil {
		return err
	}
	err = gz.Close()
	if err != nil {
		return err
	}
	// base64
	s := base64.StdEncoding.EncodeToString(bb.Bytes())

	p.CR.Status.TFState = s

	return nil
}
