package source

import (
	"github.com/go-logr/stdr"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"log"
	"os"
	"testing"
)

func testNewSources(t *testing.T) Sources {
	t.Helper()

	d, err := ioutil.TempDir("", "source_test_")
	assert.NoError(t, err)

	return Sources{
		RootPath: d,
		Log:      stdr.New(log.New(os.Stdout, "", log.Lshortfile|log.Ltime)),
	}
}

func testRemoveSources(t *testing.T, src Sources) {
	t.Helper()

	d := src.RootPath
	err := os.RemoveAll(d)
	assert.NoError(t, err)
}
