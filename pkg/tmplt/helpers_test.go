// helper functions to create, delete files during testing.
package tmplt

import (
	"github.com/otiai10/copy"
	"io/ioutil"
	"os"
	"path/filepath"
)

// Testdir holds the path of the temporary directory used to create test files in.
type testdir string

// TestdirNew creates a temporary directory for testing.
func testDirNew() testdir {
	p, err := ioutil.TempDir("", "testdir")
	if err != nil {
		panic(err)
	}
	return testdir(p)
}

// Path returns the absolute path of 'path' in the test directory.
func (tf testdir) Path(path ...string) string {
	p := append([]string{string(tf)}, path...)
	return filepath.Join(p...)
}

// MustRemove removes the file at 'path' from the test directory.
func (tf testdir) MustRemove(path string) {
	ap := tf.Path(path)
	err := os.Remove(ap)
	if err != nil {
		panic(err)
	}
}

// MustRemoveAll removes the test directory.
func (tf testdir) MustRemoveAll() {
	err := os.RemoveAll(string(tf))
	if err != nil {
		panic(err)
	}
}

// MustCreate create a file at 'path' with content 'text' in the test directory.
func (tf testdir) MustCreate(path, text string) {
	if path == "" {
		return
	}
	p := tf.Path(path)
	d := filepath.Dir(p)
	err := os.MkdirAll(d, 0700)
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile(p, []byte(text), 0600)
	if err != nil {
		panic(err)
	}
}

// MustCopy recursive copy of src files to dst in test directory.
func (tf testdir) MustCopy(src, dst string) {
	err := copy.Copy(src, tf.Path(dst))

	if err != nil {
		panic(err)
	}
}
