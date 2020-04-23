// Package source maintains local copies of (remote) repositories.
package source

import (
	"crypto/sha1"
	"fmt"
	"github.com/go-logr/logr"
	v1 "github.com/mmlt/environment-operator/api/v1"
	otia10copy "github.com/otiai10/copy"
	"hash"
	"io"
	"k8s.io/apimachinery/pkg/types"
	"os"
	"path/filepath"
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
	// BasePath is the root of all "work" and "git" directories.
	// Work directories are used by the steps, for example "work/namespace/name/infra" "work/namespace/name/clustername"
	// Git directories contain the local
	BasePath string

	// Users contains all the source users.
	// In this context an user is infra, a cluster or a test step.
	users map[userID]user

	// Srcs tracks all the sources used by users (N users can refer to 1 src).
	srcs []*src

	Log logr.Logger
}

type userID struct {
	types.NamespacedName
	user string
}

type user struct {
	// Path of the work directory.
	path string
	src  *src
}

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

func (ss *Sources) Hash(nsn types.NamespacedName, name string) (hash.Hash, error) {
	id := userID{nsn, name}

	u, ok := ss.users[id]
	if !ok {
		return nil, fmt.Errorf("source user not found: %s", name)
	}

	if timeNow().Before(u.src.lastUpdateTime.Add(time.Minute)) {
		return u.src.lastUpdateHash, nil
	}

	var h hash.Hash
	switch u.src.spec.Type {
	case v1.SourceTypeGIT:
		//TODO implement git source
	case v1.SourceTypeLocal:
		var err error
		h, err = hashDir(u.src.spec.URL)
		if err != nil {
			return nil, fmt.Errorf("calculating hash: %w", err)
		}
	}

	u.src.lastUpdateTime = timeNow()
	u.src.lastUpdateHash = h

	return h, nil
}

func (ss *Sources) Get(nsn types.NamespacedName, name string) (string, hash.Hash, error) {
	id := userID{nsn, name}

	u, ok := ss.users[id]
	if !ok {
		return "", nil, fmt.Errorf("source user not found: %s", name)
	}

	switch u.src.spec.Type {
	case v1.SourceTypeGIT:
		//TODO implement git source
	case v1.SourceTypeLocal:
		//TODO remove unused files from workdir
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

// WorkdirForName creates a workdir and returns its path.
func (ss *Sources) workdirForID(id userID) (string, error) {
	p := filepath.Join(ss.BasePath, "work", id.Namespace, id.Name, id.user)

	return p, os.MkdirAll(p, 0755)
}

// HashDir returns a hash calculated over the directory tree rooted at path.
func hashDir(path string) (hash.Hash, error) {
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
