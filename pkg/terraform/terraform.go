package terraform

import (
	"bytes"
	"encoding/json"
	"github.com/go-logr/logr"
	"github.com/hashicorp/hcl/hcl/printer"
	"github.com/hashicorp/hcl/json/parser"
	"github.com/mmlt/environment-operator/pkg/plan"
	"github.com/mmlt/environment-operator/pkg/util/exe"
	"github.com/mmlt/kubectl-tmplt/pkg/expand"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type T struct {
	log logr.Logger
}

/*
Provision(plan)
# Repo has changed and/or values (overrides) have changed.
- Use plan.clusters to write tfvars file
- Use values to expand main.tf
- Read tfstate from status.tfState
- Run terraform init
- Run terraform plan
If > N changes abort (with condition message)
- Run terraform apply
- Write tfstate back to status.tfState
- Write source SHA to status.tfSHA (optional)
- Read tf variables and update status.cluster and status.cluster.user fields
*/

func (t *T) Execute(plan *plan.Plan) error {
	//goal: not use plan outside this method

	// Get infrastructure source file(s).
	src, err := plan.InfrastructureSource()
	if err != nil {
		return err
	}
	err = src.Update()
	if err != nil {
		return err
	}

	// Write tfvars file containing default values to repo dir.
	b, err := interface2hcl(plan.InfrastructureValues().Defaults)
	if err != nil {
		return err
	}
	ioutil.WriteFile(filepath.Join(src.RepoDir(), "main.tfvars"), b, 0644)
	if err != nil {
		return err
	}

	// Expand main.tf.tmplt into main.tf
	in := filepath.Join(src.RepoDir(), plan.CR.Spec.Defaults.Infrastructure.Main)
	err = t.ExpandTmplt(in, plan.InfrastructureValues())
	if err != nil {
		return err
	}

	// Write tfstate file to infrastructure source repo dir.
	b, err = plan.GetTFState()
	if err != nil {
		return err
	}
	ioutil.WriteFile(filepath.Join(src.RepoDir(), "terraform.tfstate"), b, 0644)
	if err != nil {
		return err
	}

	// Terraform init
	err = t.TerraformInit(src.RepoDir())
	if err != nil {
		return err
	}

	//TODO
	// terraform plan (if changes > N changes abort)
	// terraform apply

	// Save tfstate.
	b, err = ioutil.ReadFile(filepath.Join(src.RepoDir(), "terraform.tfstate"))
	if err != nil {
		return err
	}
	plan.PutTFState(b)

	return nil
}

// ExpandTmplt applies text/template when file name ends in ".tmplt".
// The output is written to a file in the same dir without ".tmplt" suffix.
func (t *T) ExpandTmplt(name string, values *plan.Values) error {
	const suffix = ".tmplt"

	if !strings.HasSuffix(name, suffix) {
		return nil
	}

	ib, err := ioutil.ReadFile(name)
	if err != nil {
		return err
	}

	ob, err := expand.Run(os.Environ(), "", ib, values2yamlx(values))
	if err != nil {
		return err
	}

	return ioutil.WriteFile(strings.TrimSuffix(name, suffix), ob, 0664)
}

// TerraformInit
func (t *T) TerraformInit(dir string) error {
	_, _, err := exe.Run(t.log, &exe.Opt{Dir: dir}, "", "terraform", "init", "-input=false", "-no-color")
	if err != nil {
		return err
	}
	//TODO parse output

	return nil
}

// TerraformPlan
func (t *T) TerraformPlan(dir string) error {
	_, _, err := exe.Run(t.log, &exe.Opt{Dir: dir}, "", "terraform", "plan",
		"-out=newplan", "-var-file=main.tfvars", "-detailed-exitcode", "-input=false", "-no-color")
	// -detailed-exitcode
	//	0 = Succeeded with empty diff (no changes)
	//	1 = Error
	//	2 = Succeeded with non-empty diff (changes present)
	if err != nil {
		return err
	}

	//TODO parse std-out

	return nil
}

// TerraformApply
func (t *T) TerraformApply(dir string) error {
	_, _, err := exe.Run(t.log, &exe.Opt{Dir: dir}, "", "terraform", "apply",
		"-auto-approve", "-input=false", "-no-color", "newplan")
	if err != nil {
		return err
	}
	//TODO parse output

	return nil
}

func values2yamlx(in *plan.Values) map[string]interface{} {
	r := map[string]interface{}{}
	r["defaults"] = mss2msi(in.Defaults)
	r["clusters"] = smss2smsi(in.Clusters)
	return r
}

func smss2smsi(in []map[string]string) []map[string]interface{} {
	r := []map[string]interface{}{}
	for _, m := range in {
		r = append(r, mss2msi(m))
	}
	return r
}

func mss2msi(in map[string]string) map[string]interface{} {
	r := map[string]interface{}{}
	for k, v := range in {
		r[k] = v
	}
	return r
}

// Interface2HCL takes any structure and returns the HCL formatted form.
func interface2hcl(data interface{}) ([]byte, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return json2hcl(b)
}

// JSON2HCL takes a JSON formatted text and returns the HCL formatted equivalent.
func json2hcl(json []byte) ([]byte, error) {
	ast, err := parser.Parse(json)
	if err != nil {
		return nil, err
	}
	var bb bytes.Buffer
	err = printer.Fprint(&bb, ast)

	return bb.Bytes(), err
}
