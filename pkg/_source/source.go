// Package source provides content from sources like git repos.
package _source

import (
	"fmt"
	"github.com/go-logr/logr"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"hash/fnv"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
)

// CommonSource is what all *Source have in common.
type commonSource struct {
	// Spec is the spec of the source repo.
	spec *v1.SourceSpec

	// TempDir is the file path were the Content is stored.
	tempDir string

	// Log is the repo specific logger.
	log logr.Logger
}

// LocalSource is a local file system source.
type LocalSource struct {
	commonSource
}

// GitSource is a GIT repository source.
type GitSource struct {
	commonSource
}

// New returns a source for a given spec.
func New(spec *v1.SourceSpec, log logr.Logger) Source {
	var src Source
	switch spec.Type {
	case v1.SourceTypeGIT:
		src = &GitSource{commonSource{
			spec: spec,
			log:  log,
		}}
	case v1.SourceTypeLocal:
		src = &LocalSource{commonSource{
			spec: spec,
			log:  log,
		}}
	default:
		log.Error(nil, "unknown source type", "type", spec.Type)
	}
	return src
}

// TODO move to git.go

// Name returns the fully qualified name of the content in alphanumerical chars.
// It includes the repo URL and Ref.
// The last url path element is included verbatim.
func (c *GitSource) FQName() string {
	// url part is max 24 chars incl. 8 chars hash.
	const max = 24 - 8

	h := fnv.New32a()
	h.Write([]byte(c.spec.URL))
	h.Write([]byte(c.spec.Ref))

	b := path.Base(c.spec.URL)
	l := len(b)
	if l > max {
		l = max
	}
	return fmt.Sprintf("src-%s-%x", b[len(b)-l:], h.Sum32())
}

// TODO move to local.go

func (c *LocalSource) FQName() string {
	return c.spec.Name
}

// ReadFile reads a file with name from the Content directory.
func (c *commonSource) ReadFile(name string) ([]byte, error) {
	return ioutil.ReadFile(filepath.Join(c.tempDir, name))
}

// WriteFile writes a file with name and data in Content directory.
func (c *commonSource) WriteFile(name string, data []byte) error {
	return ioutil.WriteFile(filepath.Join(c.tempDir, name), data, 0644)
}

// AssertTempDir makes sure there is a temporary directory.
func (c *commonSource) assertTempDir(name string) error { //TODO review ifv we need this
	if c.tempDir != "" {
		return nil
	}

	p := filepath.Join(os.TempDir(), name)
	err := os.MkdirAll(p, 0755)
	if err != nil {
		return err
	}
	c.tempDir = p
	c.log.V(2).Info("Created dir", "path", p)

	return nil
}
