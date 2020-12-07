package fs

import (
	"fmt"
	"github.com/go-logr/logr"
	copy2 "github.com/otiai10/copy"
	"hash/fnv"
	"os"
	"path"
	"path/filepath"
)

// Repo represents a local copy of a file system.
type Repo struct {
	// name of repo.
	//name string
	// url is the path of the local filesystem that contains the source.
	url string
	// tempDir is the file path were the repository is kept.
	tempDir string

	// Log is the repo specific logger.
	log logr.Logger
}

// New creates a local environment at dir to Get data from an url filesystem path.
func New(url, dir string, log logr.Logger) (*Repo, error) {
	//name := path.Base(url)
	r := Repo{
		//name: name,
		url: url,
		//log:       log.WithName("FS").WithValues("repo", name),
	}
	r.log = log.WithName("FS").WithValues("repo", r.Name())

	// Create directory to clone the repo into.
	p := filepath.Join(dir, r.Name())
	err := os.MkdirAll(p, 0755)
	if err != nil {
		return nil, err
	}
	r.tempDir = p
	r.log.V(2).Info("Create dir", "path", p)

	return &r, nil
}

// Name is the fully qualified name of the repo in alphanumerical chars.
// It includes the repo URL.
// The last url path element is included verbatim.
func (r *Repo) Name() string {
	// url part is max 24 chars incl. 8 chars hash.
	const max = 24 - 8

	h := fnv.New32a()
	h.Write([]byte(r.url))

	b := path.Base(r.url)
	l := len(b)
	if l > max {
		l = max
	}
	return fmt.Sprintf("src-%s-%x", b[len(b)-l:], h.Sum32())
}

// Update refreshes the repo to the latest commit.
func (r *Repo) Update() error {
	err := os.MkdirAll(r.RepoDir(), 0755)
	if err != nil {
		return err
	}

	// source files are relative to current working directory.
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	src := filepath.Join(wd, r.url)

	r.log.Info("Copy repo", "src", src, "dest", r.RepoDir())
	return copy2.Copy(src, r.RepoDir())
}

// RepoDir returns the absolute path to the repo root.
func (r *Repo) RepoDir() string {
	return filepath.Join(r.tempDir, path.Base(r.url))
}

// Remove removes the temporary directory that contains the repository.
func (r *Repo) Remove() error {
	r.log.V(2).Info("Remove dir", "path", r.tempDir)
	return os.RemoveAll(r.tempDir)
}
