// Get content from remote GIT repository.
package git

// Package repogit provides a simple wrapper around git cli.
// Environment $HOME is expected to have .ssh/ directory to authenticate against remote repo.

import (
	"errors"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/mmlt/environment-operator/pkg/util/exe"
	"hash/fnv"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Repo represents the GIT repository to clone/pull from.
type Repo struct {
	// url of repo.
	url string
	// reference to repo content to get (see https://git-scm.com/docs/git-show-ref)
	reference string
	// tempDir is the file path were the repository is cloned.
	tempDir string

	// Log is the repo specific logger.
	log logr.Logger
}

// New creates a local environment at dir to Get data from a GIT repo at url/reference.
// Argument reference can be master, refs/heads/my-branch etc, see https://git-scm.com/docs/git-show-ref
func New(url, reference, token, dir string, log logr.Logger) (*Repo, error) {
	//name := path.Base(url)
	r := Repo{
		//name: name,
		// As token is part of the url a change of token results in a new environment.
		// This way we don't have update and existing environment when a token changes,
		// Drawback is that a long running Pod accumulates repo clones that won't be used anymore.
		url:       urlWithToken(url, token),
		reference: reference,
		log:       log.WithName("Git").WithValues("repo", path.Base(url)),
	}

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
// It includes a hash of the repo URL and Ref.
// The last url path element is included verbatim.
func (r *Repo) Name() string {
	// url part is max 24 chars incl. 8 chars hash.
	const max = 24 - 8

	h := fnv.New32a()
	_, _ = h.Write([]byte(r.url))
	_, _ = h.Write([]byte(r.reference))

	b := path.Base(r.url)
	l := len(b)
	if l > max {
		l = max
	}
	return fmt.Sprintf("src-%s-%x", b[len(b)-l:], h.Sum32())
}

// Update refreshes the repo to the latest commit.
func (r *Repo) Update() error {
	same, err := r.sameSHA()
	if err != nil {
		return err
	}
	if same {
		// Already up-to-date
		return nil
	}

	return r.Get()
}

// RepoDir returns the absolute path to the repo root.
func (r *Repo) RepoDir() string {
	return filepath.Join(r.tempDir, path.Base(r.url))
}

// Remove removes the temporary directory that contains the cloned repository.
func (r *Repo) Remove() error {
	r.log.V(2).Info("Remove dir", "path", r.tempDir)
	return os.RemoveAll(r.tempDir)
}

// SHAremote returns the SHA of the last commit to the remote repo.
func (r *Repo) SHAremote() (string, error) {
	args := []string{"ls-remote", r.url}
	if r.reference != "" {
		args = append(args, r.reference)
	}
	o, _, err := exe.Run(r.log, nil, "", "git", args...)
	if err != nil {
		return "", err
	}

	// parse result
	//  Warning: Permanently added the RSA host key for IP address '11.22.33.44' to the list of known hosts.
	//  a3a053fb28df45e33db1b634c1a45cb76e3d8bdf	refs/heads/master
	ss := strings.Fields(o)
	n := len(ss)
	if n == 0 {
		return "", fmt.Errorf("no commit for ref: %s", r.reference)
	}
	if n >= 2 && len(ss[n-2]) < 30 {
		return "", fmt.Errorf("sha of at least 30 chars expected, got: %s", o)
	}

	return ss[n-2], nil
}

// SHAlocal returns the SHA of the last commit to the local repo.
func (r *Repo) SHAlocal() (string, error) {
	args := []string{"rev-parse"}
	if r.reference != "" {
		args = append(args, r.reference)
	}
	o, _, err := exe.Run(r.log, r.optRepoDir(), "", "git", args...)
	if err != nil {
		return "", err
	}

	return strings.TrimRight(o, "\n\r"), nil
}

// Get (clone or pull) the contents of the remote repo.
func (r *Repo) Get() error {
	_, err := os.Stat(filepath.Join(r.RepoDir(), ".git"))
	if os.IsNotExist(err) {
		// repo not cloned yet
		_, _, err = exe.Run(r.log, r.optTempDir(), "", "git", "clone", r.url)
		if err != nil {
			return err
		}

		_, _, err = exe.Run(r.log, r.optTempDir(), "", "git", "checkout", r.reference)
		if err != nil {
			return err
		}
	} else {
		// repo already cloned
		_, _, err = exe.Run(r.log, r.optTempDir(), "", "git", "pull", "origin", r.reference)
		if err != nil {
			return err
		}
	}

	sha, err := r.SHAlocal()
	if err != nil {
		return err
	}
	r.log.V(1).Info("Clone/pull", "commit", sha)

	return nil
}

// SameSHA returns true when the SHA's of the local and remote GIT repo are the same.
func (r *Repo) sameSHA() (bool, error) {
	lsha, err := r.SHAlocal()
	if err != nil {
		var e *os.PathError
		if errors.As(err, &e) {
			// any PathError is interpreted as; git directory does not exists
			return false, nil
		}
		return false, err
	}
	rsha, err := r.SHAremote()
	if err != nil {
		return false, err
	}

	return lsha == rsha, nil
}

// Fingerprint returns an opaque string that's unique for the repository state.
func (r *Repo) fingerprint() (string, error) {
	s, err := r.SHAlocal()
	if err != nil {
		return "", err
	}
	return r.Name() + s, nil
}

// URLToken merges an optional token into url that starts with 'https://'
func urlWithToken(url, token string) string {
	const prefix = "https://"
	if !strings.HasPrefix(url, prefix) {
		// no action needed
		return url
	}
	return prefix + token + "@" + url[len(prefix):]
}

// OptTempDir returns options to exe commands in the temporary directory.
func (r *Repo) optTempDir() *exe.Opt {
	return &exe.Opt{Dir: r.tempDir}
}

// OptRepoDir returns options to exe commands in the repository root directory.
func (r *Repo) optRepoDir() *exe.Opt {
	return &exe.Opt{Dir: r.RepoDir()}
}
