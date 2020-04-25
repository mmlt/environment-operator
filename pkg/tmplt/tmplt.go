package tmplt

import (
	"fmt"
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

	err = Expand(string(in), out, values)
	if err != nil {
		return fmt.Errorf("%s: %w", in, err)
	}

	return nil
}

// Expand takes an in string with https://golang.org/pkg/text/template/ directives and values
// and writes the result to out.
func Expand(in string, out io.Writer, values interface{}) error {
	t, err := template.New("tmplt").Parse(in)
	if err != nil {
		return err
	}

	return t.Execute(out, values)
}





