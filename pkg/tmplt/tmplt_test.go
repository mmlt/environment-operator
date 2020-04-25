package tmplt

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestExpandAll(t *testing.T) {
	// create testdata
	td := testDirNew()
	defer td.MustRemoveAll()
	td.MustCopy("./testdata", "")

	err := ExpandAll(td.Path(), ".tmplt", nil)
	assert.NoError(t, err)

	assert.FileExists(t, td.Path("aaa.yaml"))
	assert.FileExists(t, td.Path("another_dir_level", "bbb.txt"))
}

func TestExpand(t *testing.T) {
	type Xyz struct {
		Name string
	}

	type Val struct {
		Field string
		Map map[string]string
		Struct Xyz
	}

	tests := []struct {
		it      string
		inText  string
		inValues interface{}
		want string
	}{
		{
			it: "should expand a field",
			inText: "{{ .Field }}",
			inValues: Val{
				Field: "field",
			},
			want: "field",
		},{
			it: "should expand a map",
			inText: "{{ .Map.two }}",
			inValues: Val{
				Map: map[string]string{"one": "1", "two": "2"},
			},
			want: "2",
		},{
			it: "should expand a struct",
			inText: "{{ .Struct.Name }}",
			inValues: Val{
				Struct: Xyz{Name: "xyzfield"},
			},
			want: "xyzfield",
		},
	}
	for _, tst := range tests {
		t.Run(tst.it, func(t *testing.T) {
			out := &bytes.Buffer{}
			err := Expand(tst.inText, out, tst.inValues)
			assert.NoError(t, err)
			got := out.String()
			assert.Equal(t, tst.want, got)
		})
	}
}