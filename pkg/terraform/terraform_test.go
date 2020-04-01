package terraform

import (
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/plan"
	"github.com/mmlt/testr"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"
)

func TestInterface2HCL(t *testing.T) {
	in := map[string]interface{}{
		"one": 1,
		"two": "twee",
	}
	want :=
		`"one" = 1

"two" = "twee"`
	got, err := interface2hcl(in)
	assert.NoError(t, err)
	assert.Equal(t, want, string(got))
}

//TODO consider moving this test to plan package and/or splitting it as it tests many things; plan, source, terraform
// or should it be called TestExecute?
func TestExpandTmplt(t *testing.T) {
	testr.SetVerbosity(5)
	log := testr.New(t)
	p := plan.New(&v1.Environment{
		Spec: v1.EnvironmentSpec{
			Defaults: v1.ClusterSpec{
				Infrastructure: v1.InfrastructureSpec{
					Source: v1.SourceSpec{
						Type: "local",
						URL:  "testdata", // relative to dir containing this _test.go file.
					},
					Main: "main.tf.tmplt",
					Values: map[string]string{
						"first": "default",
					},
				},
			},
			Clusters: []v1.ClusterSpec{
				{
					Name: "cpe",
					Infrastructure: v1.InfrastructureSpec{
						Values: map[string]string{
							"first": "cluster",
						},
					},
				}, {
					Name: "second",
					Infrastructure: v1.InfrastructureSpec{
						Values: map[string]string{
							"first": "cluster",
						},
					},
				},
			},
		},
	}, log)

	// Get source files.
	src, err := p.InfrastructureSource()
	assert.NoError(t, err)
	defer src.Remove()
	err = src.Update()
	assert.NoError(t, err)

	// Expand in-file to out-file.
	inFile := filepath.Join(src.RepoDir(), p.CR.Spec.Defaults.Infrastructure.Main)
	outFile := strings.TrimSuffix(inFile, ".tmplt")
	tf := &T{
		log: log.WithName("tf"),
	}
	err = tf.ExpandTmplt(inFile, p.InfrastructureValues())
	assert.NoError(t, err)

	// Check out-file against golden copy.
	assert.FileExists(t, outFile)
	want, err := ioutil.ReadFile("testdata/main.tf.want")
	assert.NoError(t, err)
	got, err := ioutil.ReadFile(outFile)
	assert.NoError(t, err)
	assert.Equal(t, string(want), string(got))
}
