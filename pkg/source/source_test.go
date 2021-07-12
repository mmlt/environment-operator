package source

import (
	v1 "github.com/mmlt/environment-operator/api/v1"
	otia10copy "github.com/otiai10/copy"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	metav1 "k8s.io/apimachinery/pkg/types"
	"os"
	"path/filepath"
	"testing"
)

var nsn = metav1.NamespacedName{
	Namespace: "default",
	Name:      "test",
}

// TestSources Type: "local" when URL (containing path to local source directory) is changed.
// Show workdir: tree /tmp/source_test_*
func TestSources_e2e_local_spec_change(t *testing.T) {
	mutations := []struct {
		comment     string
		source      string
		wantChanged bool
	}{
		{
			comment:     "init",
			source:      "testdata/step1",
			wantChanged: true,
		},
		{
			comment:     "new content",
			source:      "testdata/step2",
			wantChanged: true,
		},
		{
			comment:     "same content, no change expected",
			source:      "testdata/step2",
			wantChanged: false,
		},
	}

	name := "clusterxyz"

	ss := testNewSources(t)
	defer testRemoveSources(t, ss)

	for _, mutation := range mutations {
		// create spec that will change each step.
		spec := v1.SourceSpec{
			Type: "local",
			URL:  mutation.source,
		}

		err := ss.Register(nsn, name, spec)
		if !assert.NoError(t, err, "Register") {
			return
		}

		err = ss.FetchAll()
		if !assert.NoError(t, err, "FetchAll") {
			return
		}

		gotChanged, err := ss.Get(nsn, name)
		if !assert.NoError(t, err, "Get") {
			return
		}

		assert.Equal(t, mutation.wantChanged, gotChanged, "<changed> at mutation '%s'", mutation.comment)
	}

	assert.FileExists(t, filepath.Join(ss.RootPath, "workspace", nsn.Namespace, nsn.Name, name, "content", "file2.txt"))

	// currently the workspace directories aren't pruned, that's why file1.txt still exists.
	assert.FileExists(t, filepath.Join(ss.RootPath, "workspace", nsn.Namespace, nsn.Name, name, "content", "file1.txt"))
}

// TestSources Type: "local" when the content of the local source directory is changed.
// Show workdir: tree /tmp/source_test_*
func TestSources_e2e_local_content_change(t *testing.T) {
	mutations := []struct {
		comment  string
		testdata string
		// the testdata path to check for change.
		area        string
		wantChanged bool
	}{
		{
			comment:     "init",
			testdata:    "testdata/step1",
			area:        "content",
			wantChanged: true,
		},
		{
			comment:     "change content",
			testdata:    "testdata/step2",
			area:        "content",
			wantChanged: true,
		},
		{
			comment:     "change content outside area",
			testdata:    "testdata/step3",
			area:        "content",
			wantChanged: false,
		},
	}

	name := "clusterxyz"

	ss := testNewSources(t)
	defer testRemoveSources(t, ss)

	// create a tmp local source dir that will mutated later on.
	src, err := ioutil.TempDir("", "source_test_")
	assert.NoError(t, err)
	defer assert.NoError(t, os.RemoveAll(src))

	for _, mutation := range mutations {
		spec := v1.SourceSpec{
			Type: "local",
			URL:  src,
			Area: mutation.area,
		}

		// change source content
		err = otia10copy.Copy(mutation.testdata, src)
		assert.NoError(t, err)

		err = ss.Register(nsn, name, spec)
		if !assert.NoError(t, err, "Register") {
			return
		}

		err = ss.FetchAll()
		if !assert.NoError(t, err, "FetchAll") {
			return
		}

		gotChanged, err := ss.Get(nsn, name)
		if !assert.NoError(t, err, "Get") {
			return
		}

		assert.Equal(t, mutation.wantChanged, gotChanged, "<changed> at mutation '%s'", mutation.comment)
	}

	assert.FileExists(t, filepath.Join(ss.RootPath, "workspace", nsn.Namespace, nsn.Name, name, "content", "file2.txt"))

	// currently the workspace directories aren't pruned, that's why file1.txt still exists.
	assert.FileExists(t, filepath.Join(ss.RootPath, "workspace", nsn.Namespace, nsn.Name, name, "content", "file1.txt"))
}
