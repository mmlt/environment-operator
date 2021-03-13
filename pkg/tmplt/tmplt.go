package tmplt

import (
	"fmt"
	"github.com/Masterminds/sprig/v3"
	"github.com/rodaine/hclencoder"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// ExpandAll looks for files in path directory and its subdirectories with suffix, expands them
// and writes the result in a file without suffix.
func ExpandAll(path, suffix string, values interface{}) error {
	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if !strings.HasSuffix(path, suffix) {
			return nil
		}
		if info.IsDir() {
			return nil
		}

		return ExpandFile(path, suffix, values)
	})
	return err
}

// ExpandFile expands the file at path and writes the result in a file without suffix.
func ExpandFile(path, suffix string, values interface{}) error {
	in, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	out, err := os.Create(strings.TrimSuffix(path, suffix))
	if err != nil {
		return err
	}
	defer out.Close()

	err = Expand(path, string(in), out, values)
	if err != nil {
		return err
	}

	return out.Close()
}

// Expand takes an in string with https://golang.org/pkg/text/template/ directives and values
// and writes the result to out.
func Expand(name, in string, out io.Writer, values interface{}) error {
	funcMap := sprig.TxtFuncMap()

	// add extra functionality
	funcMap["toHCL"] = toHCL

	t, err := template.New(name).Funcs(funcMap).Parse(in)
	if err != nil {
		return err
	}

	return t.Execute(out, values)
}

func toHCL(in interface{}) string {
	b, err := hclencoder.Encode(in)
	if err != nil {
		return fmt.Sprintf("toHCL: %v", err)
	}
	return string(b)
}
