// Package source maintains local copies of (remote) repositories.
package source

import (
	"crypto/sha1"
	"fmt"
	"github.com/go-logr/logr"
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

/*
Implementation notes:

             fetch                  Get()
remote repo --------> local source --------> workdir
                         Hash()

Hash and Get are not atomic.
Hash may return a value that is not equal to the hash Get is returning.
For example:
  1. local store has changed
  2. Hash returns to value for 1 and because of that NextStep will detect a change and produce a Step.
  3. local store change is undone
  4. Step executes and Get's  unchanged data
Effect: Step run on unchanged data (not an issue).

Get and update of local source should be mutual exclusive.
Updating the local source during Get copying to the workdir must be prevented.

The case of an unknown party (for example a mounted ConfigMap) updating local source is not supported (would need some file locking protocol).
The case of git fetch is solved by serializing fetch/Get.
*/

// Sources keeps a list of source repositories and maintains local copies of them.
// Sources are added with Register() and retrieved with Getter.
type Sources struct {
	// RootPath is the root of all "work" and "git" directories.
	// Work directories are used by the steps, for example "work/namespace/name/infra" "work/namespace/name/clustername"
	// Git directories contain the local
	RootPath string

	// Users contains all the source users.
	// In this context an user is infra, a cluster or a test step.
	users map[userID]user

	// Srcs tracks all the sources used by users (N users can refer to 1 src).
	srcs []*src

	Log logr.Logger
}

// UserID identifies the consumer of the source.
type userID struct {
	// NamespacedName identifies the environment.
	types.NamespacedName
	// User identifies the cluster within the environment.
	user string
}

// User (TODO rename to src?)
type user struct {
	// Path of the work directory.
	path string
	src  *src
}

// Src (TODO rename to spec?) is the meta data of a local copy of a source.
type src struct {
	spec           v1.SourceSpec
	lastUpdateTime time.Time
	lastUpdateHash hash.Hash
}

// Ninfra is a special name that is used for infra content.
const Ninfra = "_infra_"

type Getter interface {
	// Hash returns the hash of the source content.
	Hash(nsn types.NamespacedName, name string) (hash.Hash, error)
	// Get copies the source content to a workdir and returns its path.
	Get(nsn types.NamespacedName, name string) (string, hash.Hash, error)
}

// For testing.
var timeNow = time.Now

// Hash implements Getter.
func (ss *Sources) Hash(nsn types.NamespacedName, name string) (hash.Hash, error) {
	id := userID{nsn, name}

	u, ok := ss.users[id]
	if !ok {
		return nil, fmt.Errorf("source user not found: %s", name)
	}

	// Rate limit.
	if timeNow().Before(u.src.lastUpdateTime.Add(time.Minute)) {
		return u.src.lastUpdateHash, nil
	}

	var h hash.Hash
	switch u.src.spec.Type {
	case v1.SourceTypeGIT:
		var err error
		h, err = ss.gitFetch(u.src.spec)
		if err != nil {
			return nil, err
		}
	case v1.SourceTypeLocal:
		var err error
		h, err = hashAll(u.src.spec.URL)
		if err != nil {
			return nil, err
		}
	}

	u.src.lastUpdateTime = timeNow()
	u.src.lastUpdateHash = h

	return h, nil
}

// Get implements Getter.
func (ss *Sources) Get(nsn types.NamespacedName, name string) (string, hash.Hash, error) { //TODO remove hash because it's confusing, one should Hash() instead
	id := userID{nsn, name}

	u, ok := ss.users[id]
	if !ok {
		return "", nil, fmt.Errorf("source user not found: %s", name)
	}

	switch u.src.spec.Type {
	case v1.SourceTypeGIT:
		p := ss.gitPath(u.src.spec)
		// TODO sync with GIT fetch to prevent concurrent fetch and copy
		err := otia10copy.Copy(p, u.path, otia10copy.Options{
			Skip: func(p string) bool { return strings.HasSuffix(p, ".git") },
		})
		if err != nil {
			return "", nil, fmt.Errorf("source get(%s): %w", name, err)
		}
	case v1.SourceTypeLocal:
		//TODO remove unused files from workdir?
		err := otia10copy.Copy(u.src.spec.URL, u.path)
		if err != nil {
			return "", nil, fmt.Errorf("source get(%s): %w", name, err)
		}
	}

	return u.path, u.src.lastUpdateHash, nil
}

// Register name as an user of the spec source.
func (ss *Sources) Register(nsn types.NamespacedName, name string, spec v1.SourceSpec) error {
	id := userID{nsn, name}

	if ss.users == nil {
		ss.users = make(map[userID]user)
	}

	// Get src.
	s, ok := ss.srcBySpec(spec)
	if !ok {
		s = &src{spec: spec}
		ss.srcs = append(ss.srcs, s)
	}

	// Get user.
	if u, ok := ss.users[id]; ok {
		if spec.Type == u.src.spec.Type && spec.Ref == u.src.spec.Ref && spec.URL == u.src.spec.URL {
			// name/spec combination exists
			return nil
		}
		// user exists but the spec has changed.
		u.src = s

		return nil
	}

	// Create new user.
	p, err := ss.workdirForID(id)
	if err != nil {
		return err
	}
	ss.users[id] = user{
		path: p,
		src:  s,
	}

	return nil
}

// SrcBySpec return a source matching spec.
func (ss *Sources) srcBySpec(spec v1.SourceSpec) (*src, bool) {
	for i, s := range ss.srcs {
		if spec.Type == s.spec.Type && spec.Ref == s.spec.Ref && spec.URL == s.spec.URL {
			return ss.srcs[i], true
		}
	}
	return nil, false
}

// WorkdirForID creates a workdir and returns its path.
func (ss *Sources) workdirForID(id userID) (string, error) {
	p := filepath.Join(ss.RootPath, "work", id.Namespace, id.Name, id.user)

	return p, os.MkdirAll(p, 0755)
}

// GITFetch fetches content of a GIT repo and returns its hash.
// NB. the returned hash is a function of the GIT hash but not the same.
func (ss *Sources) gitFetch(spec v1.SourceSpec) (hash.Hash, error) {
	p := ss.gitPath(spec)
	_, err := os.Stat(p)
	if os.IsNotExist(err) {
		// Clone new repo.
		d, _ := filepath.Split(p)
		err = os.MkdirAll(d, 0775)
		if err != nil {
			return nil, err
		}

		_, _, err = exe.Run(ss.Log, &exe.Opt{Dir: d}, "", "git", "clone", spec.URL)
		if err != nil {
			return nil, err
		}

		_, _, err = exe.Run(ss.Log, &exe.Opt{Dir: p}, "", "git", "checkout", spec.Ref)
		if err != nil {
			return nil, err
		}
		ss.Log.Info("GIT-clone", "url", spec.URL, "ref", spec.Ref)
	} else {
		// Pull existing repo content.
		_, _, err = exe.Run(ss.Log, &exe.Opt{Dir: p}, "", "git", "pull", "origin", spec.Ref)
		if err != nil {
			return nil, err
		}
		ss.Log.V(2).Info("GIT-pull", "url", spec.URL, "ref", spec.Ref)
	}

	// Get hash.
	o, _, err := exe.Run(ss.Log, &exe.Opt{Dir: p}, "", "git", "rev-parse")
	if err != nil {
		return nil, err
	}
	o = strings.TrimRight(o, "\n\r")
	h := sha1.New()
	h.Write([]byte(o))

	return h, nil
}

// GITPath returns a path to a local GIT repo.
// The path is in the form /basepath/git/url/ref/name
// (name is base element of the URL, url and ref elements are mangled)
func (ss *Sources) gitPath(spec v1.SourceSpec) string {
	// url part is an 8 chars hash followed by the URL base up to 24 chars in total.
	const max = 24 - 8
	h := fnv.New32a()
	h.Write([]byte(spec.URL))
	b := path.Base(spec.URL)
	l := len(b)
	if l > max {
		l = max
	}
	u := fmt.Sprintf("%s-%x", b[len(b)-l:], h.Sum32())

	r := strings.ReplaceAll(spec.Ref, "/", "")

	return filepath.Join(ss.RootPath, "git", u, r, b)
}

// HashAll returns a hash calculated over the directory tree rooted at path.
func hashAll(path string) (hash.Hash, error) {
	h := sha1.New()
	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		io.WriteString(h, path)

		if info.IsDir() {
			return nil
		}

		r, err := os.Open(path)
		if err != nil {
			return err
		}
		_, err = io.Copy(h, r)
		r.Close()
		if err != nil {
			return err
		}

		return nil
	})

	return h, err
}
