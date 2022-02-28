// Package source maintains local copies of (remote) repositories.
package source

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"github.com/go-logr/logr"
	multierror "github.com/hashicorp/go-multierror"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/util/exe"
	otia10copy "github.com/otiai10/copy"
	"hash"
	"hash/fnv"
	"io"
	"k8s.io/apimachinery/pkg/types"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// Usage of this package involves the following steps:
//	1. Workspace are Registered - typically each infra and cluster config has its own workspace directory.
//	2. Remote repos (or filesystems) are fetched.
//	3. When no steps are running the workspace sources are 'get' from the local repo.
//	4. Steps run commands in the workspace directories.
//
//                 fetch             get
// 	remote repo  --------->  repo  ------->  workspace
// 	filesystem
//
// Change of filesystem during fetch might result in inconsistencies in repo content.

// Sources keeps a list of workspaces and associated the source repositories.
// Sources are added with Register().
type Sources struct {
	// RootPath is the root of "workspace" and "repo" directories.
	// Workspaces have paths like "<RootPath>/workspace/namespace/name/_infra_" "work/namespace/name/clustername"
	// Repo directory are in "<RootPath>/repo/"
	RootPath string

	// Workspaces map consumers to workspaces.
	// In this context a consumer is an infra or cluster deployment configuration step.
	workspaces map[consumerID]Workspace

	// Repos keeps tack of the remote repos/filesystems.
	repos map[v1.SourceSpec]repo

	Log logr.Logger
}

// ConsumerID identifies the consumer of a workspace.
type consumerID struct {
	// NamespacedName identifies the environment.
	types.NamespacedName
	// consumer identifies the cluster within the environment or the environment itself.
	consumer string
}

// Workspace is a copy of a repo dedicated to a consumer.
type Workspace struct {
	// Path of the work directory.
	Path string
	// Spec of the required repo.
	Spec v1.SourceSpec
	// Hash of the content (limited to area).
	Hash string
	// Synced is true if the repo content is copied to the workspace.
	// Synced is false as long as a repo hasn't been fetched or Get() isn't called or Get() has been called but new repo
	// content is fetched.
	Synced bool
}

// Repo represents a local copy of a remote (GIT) repo or filesystem.
type repo struct {
	// LastFetched is the last time a repo has been fetched.
	lastFetched time.Time

	// Hash is the hash of the fetched content.
	hash string
}

// Register nsn + name as requiring a workspace with spec content.
func (ss *Sources) Register(nsn types.NamespacedName, name string, spec v1.SourceSpec) error {
	name = defaultName(name)

	id := consumerID{nsn, name}

	if ss.workspaces == nil {
		ss.workspaces = make(map[consumerID]Workspace)
	}

	// Check for existing workspace.
	if w, ok := ss.workspaces[id]; ok {
		if spec.Type == w.Spec.Type && spec.URL == w.Spec.URL && spec.Ref == w.Spec.Ref && spec.Token == w.Spec.Token {
			return nil
		}
		// workspace exists but the spec has changed.
		w.Spec = spec
		w.Synced = false
		ss.workspaces[id] = w

		return nil
	}

	// Create new workspace.
	p := ss.workspacePath(id)
	err := os.MkdirAll(p, 0750)
	if err != nil {
		return err
	}
	ss.workspaces[id] = Workspace{
		Path: p,
		Spec: spec,
	}

	return nil
}

// Get copies the source content to a workspace and returns true if the workspace has changed.
func (ss *Sources) Get(nsn types.NamespacedName, name string) (bool, error) {
	name = defaultName(name)

	id := consumerID{nsn, name}

	w, ok := ss.workspaces[id]
	if !ok {
		return false, fmt.Errorf("source: workspace not found: %s", name)
	}

	_, ok = ss.repos[w.Spec]
	if !ok {
		return false, fmt.Errorf("source: get(%s): repo not fetched yet", name)
	}

	rp := ss.repoPath(w.Spec)

	// get hash of area within repo.
	// (if we didn't care about area we could have used repo.hash)
	h, err := ss.hashAll(filepath.Join(rp, w.Spec.Area))
	if err != nil {
		return false, err
	}
	hs := hex.EncodeToString(h.Sum(nil))

	if w.Hash == hs {
		return false, nil
	}

	ss.Log.Info("Get workspace (repo changed)", "request", nsn, "name", name)

	// TODO sync with fetch to prevent inconsistent copies
	err = otia10copy.Copy(rp, w.Path, otia10copy.Options{
		Skip: func(p string) bool { return strings.HasSuffix(p, ".git") },
	})
	if err != nil {
		return false, fmt.Errorf("source: get(%s): %w", name, err)
	}

	w.Hash = hs
	w.Synced = true
	ss.workspaces[id] = w

	return true, nil
}

// Workspace returns the Workspace for a nsn + name.
// Returns false if workspace is not found.
func (ss *Sources) Workspace(nsn types.NamespacedName, name string) (Workspace, bool) {
	name = defaultName(name)

	id := consumerID{nsn, name}

	w, ok := ss.workspaces[id]

	return w, ok
}

// FetchAll fetches all remote repo's or filesystems into a local repo directory.
// The fetch rate is limited to at most once per N minutes.
func (ss *Sources) FetchAll() error {
	var errs error
	for _, w := range ss.workspaces {
		err := ss.fetch(w.Spec)
		if err != nil {
			errs = multierror.Append(errs, err)
		}
	}
	return errs
}

// For testing.
var timeNow = time.Now

// Fetch fetches a remote repo or filesystem specified by spec into a local repo directory.
// The fetch rate is limited to at most once per N minutes.
func (ss *Sources) fetch(spec v1.SourceSpec) error {
	if ss.repos == nil {
		ss.repos = make(map[v1.SourceSpec]repo)
	}

	// fetch
	var err error
	var h string
	switch spec.Type {
	case v1.SourceTypeGIT:
		h, err = ss.gitFetch(spec)
	case v1.SourceTypeLocal:
		h, err = ss.localFetch(spec)
	default:
		err = fmt.Errorf("source: unknown type: %s", spec.Type)
	}
	if err != nil {
		return err
	}

	ss.repos[spec] = repo{
		lastFetched: timeNow(),
		hash:        h,
	}

	return nil
}

// LocalFetch fetches the content of a local source like a directory and returns its hash.
func (ss *Sources) localFetch(spec v1.SourceSpec) (string, error) {
	p := ss.repoPath(spec)
	err := os.MkdirAll(p, 0750)
	if err != nil {
		return "", err
	}

	//TODO prevent target directory from accumulating unused files
	// remove all files before copy
	// or
	// walk target dir and diff with source dir

	// Copy local dir to repo path.
	// Ignore .git directory.
	err = otia10copy.Copy(spec.URL, p, otia10copy.Options{Skip: func(src string) bool {
		return filepath.Base(src) == ".git"
	}})
	if err != nil {
		return "", fmt.Errorf("fetch: %w", err)
	}

	h, err := ss.hashAll(spec.URL) // TODO use hashAll(p) when dir is properly synced (see previous to do)
	if err != nil {
		return "", err
	}
	s := hex.EncodeToString(h.Sum(nil))

	return s, err
}

// GITFetch fetches content of a GIT repo and returns its hash.
func (ss *Sources) gitFetch(spec v1.SourceSpec) (string, error) {
	p := ss.repoPath(spec)
	_, err := os.Stat(p)
	if os.IsNotExist(err) {
		// Clone new repo.
		d, _ := filepath.Split(p)
		err = os.MkdirAll(d, 0750)
		if err != nil {
			return "", err
		}

		_, _, err = exe.Run(ss.Log, &exe.Opt{Dir: d}, "", "git", "clone", urlWithToken(spec.URL, spec.Token))
		if err != nil {
			return "", err
		}

		_, _, err = exe.Run(ss.Log, &exe.Opt{Dir: p}, "", "git", "checkout", spec.Ref)
		if err != nil {
			return "", err
		}
		ss.Log.Info("GIT-clone", "url", spec.URL, "ref", spec.Ref)
	} else {
		// Pull existing repo content.
		_, _, err = exe.Run(ss.Log, &exe.Opt{Dir: p}, "", "git", "pull", "origin", spec.Ref)
		if err != nil {
			return "", err
		}
		ss.Log.V(2).Info("GIT-pull", "url", spec.URL, "ref", spec.Ref)
	}

	// Get hash.
	h, _, err := exe.Run(ss.Log, &exe.Opt{Dir: p}, "", "git", "rev-parse", spec.Ref)
	if err != nil {
		return "", err
	}
	h = strings.TrimRight(h, "\n\r")
	if len(h) == 0 {
		return "", fmt.Errorf("expected git hash")
	}

	return h, nil
}

// RepoPath returns a path to a repo.
// The path is in the form RootPath/repo/url/ref/name
// where name is base element of the URL, url and ref elements are mangled.
func (ss *Sources) repoPath(spec v1.SourceSpec) string {
	// url part is an 8 chars hash followed by the URL base up to 24 chars in total.
	const max = 24 - 8
	h := fnv.New32a()
	_, _ = h.Write([]byte(spec.URL))
	b := path.Base(spec.URL)
	l := len(b)
	if l > max {
		l = max
	}
	u := fmt.Sprintf("%s-%x", b[len(b)-l:], h.Sum32())

	r := strings.ReplaceAll(spec.Ref, "/", "")

	return filepath.Join(ss.RootPath, "repo", u, r, b)
}

// WorkspacePath returns the path to a workspace dir.
func (ss *Sources) workspacePath(id consumerID) string {
	return filepath.Join(ss.RootPath, "workspace", id.Namespace, id.Name, id.consumer)
}

// HashAll returns a hash calculated over the directory tree rooted at path.
func (ss *Sources) hashAll(path string) (hash.Hash, error) {
	h := sha1.New()
	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		_, _ = io.WriteString(h, path)

		if info.IsDir() {
			return nil
		}

		r, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func(r *os.File) {
			err := r.Close()
			if err != nil {
				ss.Log.Error(err, "hashAll")
			}
		}(r)
		_, err = io.Copy(h, r)
		if err != nil {
			return err
		}

		return nil
	})

	return h, err
}

// DefaultName returns a default for an empty name.
func defaultName(name string) string {
	if name == "" {
		return "_infra_"
	}
	return name
}

// URLWithToken merges an optional token into url that starts with 'https://'
func urlWithToken(url, token string) string {
	if token == "" {
		return url
	}

	const prefix = "https://"
	if !strings.HasPrefix(url, prefix) {
		return url
	}

	return prefix + token + "@" + url[len(prefix):]
}
