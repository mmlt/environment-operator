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
		Field  string
		Map    map[string]string
		Struct Xyz
		List   []string
		Any    interface{}
	}

	type HCL struct {
		One   string `hcl:"one"`
		Two   int    `hcl:"two"`
		Three string `hcl:"three" hcle:"omitempty"`
	}

	tests := []struct {
		it       string
		inText   string
		inValues interface{}
		want     string
	}{
		{
			it:     "should expand a field",
			inText: "{{ .Field }}",
			inValues: Val{
				Field: "field",
			},
			want: "field",
		}, {
			it:     "should expand a map",
			inText: "{{ .Map.two }}",
			inValues: Val{
				Map: map[string]string{"one": "1", "two": "2"},
			},
			want: "2",
		}, {
			it:     "should expand a struct field",
			inText: "{{ .Struct.Name }}",
			inValues: Val{
				Struct: Xyz{Name: "xyzfield"},
			},
			want: "xyzfield",
		}, {
			it:     "should expand a list raw",
			inText: "{{ .List }}",
			inValues: Val{
				List: []string{"one", "two", "three"},
			},
			want: `[one two three]`,
		}, {
			it:     "should expand any list HCL formatted",
			inText: "{{toHCL .Any }}",
			inValues: Val{
				Any: []string{"one", "two", "three"},
			},
			want: `[
  "one",
  "two",
  "three",
]
`,
		}, {
			it:     "should expand an annotated struct HCL formatted",
			inText: "{{toHCL .Any }}",
			inValues: Val{
				Any: HCL{
					One:   "one",
					Two:   2,
					Three: "333",
				},
			},
			want: `one = "one"

two = 2

three = "333"
`,
		}, {
			it:     "should expand an annotated struct HCL formatted, respecting omitempty",
			inText: "{{toHCL .Any }}",
			inValues: Val{
				Any: HCL{
					One: "one",
					Two: 2,
				},
			},
			want: `one = "one"

two = 2
`,
		},
	}
	for _, tst := range tests {
		t.Run(tst.it, func(t *testing.T) {
			out := &bytes.Buffer{}
			err := Expand("testing", tst.inText, out, tst.inValues)
			assert.NoError(t, err)
			got := out.String()
			assert.Equal(t, tst.want, got)
		})
	}
}
