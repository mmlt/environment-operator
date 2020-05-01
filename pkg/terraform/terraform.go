package terraform

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"github.com/go-logr/logr"
	"github.com/mmlt/environment-operator/pkg/util/exe"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// Terraformer is able to provision infrastructure.
type Terraformer interface {
	// Init resolves (downloads) dependencies for the terraform configuration files in dir.
	Init(dir string) *TFResult
	// Plan creates an execution plan for the terraform configuration files in dir.
	Plan(dir string) *TFResult
	// StartApply applies the plan in dir without waiting for the result.
	// If a Cmd is returned cmd.Wait() should be called wait for completion and clean-up.
	StartApply(ctx context.Context, dir string) (*exec.Cmd, chan TFApplyResult, error)
	// Output returns a map with output types and values.
	// When outputs.tf contains output "xyz" { value = 7 } the returned map contains ["yxz"]["value"] == 1
	Output(dir string) (map[string]interface{}, error)
}

// TFResults is the condensed output of a terraform command.
type TFResult struct {
	// Info > 0 means command successful.
	Info int
	// Warnings > 0 means command successful but something unexpected happened.
	Warnings int
	// Errors > 0 means the command failed.
	Errors int

	PlanAdded   int
	PlanChanged int
	PlanDeleted int
}

// TFApplyResult
type TFApplyResult struct {
	// Running count of the number of objects that have started creating, modifying and destroying.
	Creating, Modifying, Destroying int

	// Errors is a list of error messages.
	Errors []string

	// Number of object added, changed, destroyed upon completion.
	// The numbers are non-zero when terraform completed successfully.
	TotalAdded, TotalChanged, TotalDestroyed int

	// Most recently logged terraform object name.
	Object string
	// Most recently logged action being performed; creating (creation), modifying (modification), destroying (destruction).
	// *ing means in-progress, *tion means completed. TODO consider normalizing *tion to *ed
	Action string
	// Most recently logged elapsed time reported by terraform.
	Elapsed string
}

// Terraform provisions infrastructure using terraform cli.
type Terraform struct {
	Log logr.Logger
}

// Init implements Terraformer.
func (t *Terraform) Init(dir string) *TFResult {
	o, _, err := exe.Run(t.Log, &exe.Opt{Dir: dir}, "", "terraform", "init", "-input=false", "-no-color")

	return parseInitResponse(o, err)
}

func parseInitResponse(text string, err error) *TFResult {
	r := &TFResult{}
	ire := regexp.MustCompile("Terraform has been successfully initialized!")
	r.Info = len(ire.FindAllStringIndex(text, -1))
	wre := regexp.MustCompile("\nWarning: ")
	r.Warnings = len(wre.FindAllStringIndex(text, -1))
	ere := regexp.MustCompile(" errors |Terraform initialized in an empty directory!")
	r.Errors = len(ere.FindAllStringIndex(text, -1))

	if err != nil {
		r.Errors++
	}

	return r
}

// Plan implements Terraformer.
func (t *Terraform) Plan(dir string) *TFResult {
	o, _, err := exe.Run(t.Log, &exe.Opt{Dir: dir}, "", "terraform", "plan",
		"-out=newplan", "-detailed-exitcode", "-input=false", "-no-color")
	return parsePlanResponse(o, err)
}

// ParsePlanResponse parses terraform stdout text and err and returns
// Expect detailed exit codes as provided by 'terraform plan -detailed-exitcode'.
//	0 = Succeeded with empty diff (no changes)
//  1 = Error
//  2 = Succeeded with non-empty diff (changes present)
func parsePlanResponse(text string, err error) *TFResult {
	r := &TFResult{}

	pre := regexp.MustCompile(`Plan: (\d+) to add, (\d+) to change, (\d+) to destroy.`)
	ps := pre.FindAllStringSubmatch(text, -1)
	if len(ps) == 1 && len(ps[0]) == 4 {
		var ea, ec, ed error
		r.PlanAdded, ea = strconv.Atoi(ps[0][1])
		r.PlanChanged, ec = strconv.Atoi(ps[0][2])
		r.PlanDeleted, ed = strconv.Atoi(ps[0][3])
		if ea == nil && ec == nil && ed == nil {
			// only if all conditions are met we call it a success.
			r.Info = 1
		}
	}

	wre := regexp.MustCompile("\nWarning: ")
	r.Warnings = len(wre.FindAllStringIndex(text, -1))
	ere := regexp.MustCompile("\nError: ")
	r.Errors = len(ere.FindAllStringIndex(text, -1))

	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if ee.ExitCode() == 1 {
			r.Errors++
			r.Info = 0
		}

		return r
	}

	if err != nil {
		// any other error then ExitError
		r.Errors++
	}

	return r
}

// StartApply implements Terraformer.
func (t *Terraform) StartApply(ctx context.Context, dir string) (*exec.Cmd, chan TFApplyResult, error) {
	cmd := exe.RunAsync(ctx, t.Log, &exe.Opt{Dir: dir}, "", "terraform", "apply",
		"-auto-approve", "-input=false", "-no-color", "newplan")

	o, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}

	cmd.Stderr = cmd.Stdout // combine

	err = cmd.Start()
	if err != nil {
		return nil, nil, err
	}

	ch := t.parseAsyncApplyResponse(o)

	return cmd, ch, nil
}

// ParseAsyncApplyResponse parses in and returns results when interesting input is encountered.
// Close in to release the go func.
func (t *Terraform) parseAsyncApplyResponse(in io.ReadCloser) chan TFApplyResult {
	out := make(chan TFApplyResult)

	// hold running totals.
	result := &TFApplyResult{}

	go func() {
		sc := bufio.NewScanner(in)
		for sc.Scan() {
			s := sc.Text()
			t.Log.V(3).Info("RunAsync-result", "text", s)
			r := parseApplyResponseLine(result, s)
			if r != nil {
				out <- *r
			}
		}
		if err := sc.Err(); err != nil {
			t.Log.Error(err, "parseAsyncApplyResponse")
		}

		close(out)
	}()

	return out
}

// ParseApplyResponseLine parses a line.
// If content can be extracted from line it returns an updated shallow copy of in, otherwise it returns nil.
// It increments in running counters.
func parseApplyResponseLine(in *TFApplyResult, line string) *TFApplyResult {
	ss := strings.Split(line, " ")
	if len(ss) < 2 {
		// not interesting.
		return nil
	}

	r := *in

	if ss[0] == "Error:" {
		r.Errors = append(r.Errors, line[len("Error: "):])
		return &r
	}

	if strings.HasSuffix(ss[0], ":") {
		r.Object = ss[0][:len(ss[0])-1]

		if strings.HasSuffix(ss[1], "...") {
			// xyz.this: Creating...
			a := normalizeAction(ss[1])
			switch a {
			case "creating":
				in.Creating++
			case "modifying":
				in.Modifying++
			case "destroying":
				in.Destroying++
			}
			// 'in' is modified, make a new shallow copy.
			o := r.Object
			r = *in
			r.Object = o
			r.Action = a
			return &r
		}

		if len(ss) > 3 {
			if ss[1] == "Still" && strings.HasSuffix(ss[2], "...") {
				// xyz.this: Still creating... [10s elapsed]
				r.Action = normalizeAction(ss[2])
				r.Elapsed = ss[len(ss)-2]
				return &r
			}

			if ss[2] == "complete" {
				// xyz.this: Creation complete after 6m22s [id=/subscriptions/.../x]
				r.Action = normalizeAction(ss[1])
				r.Elapsed = ss[4]
				return &r
			}
		}
	}

	if ss[0] == "Apply" {
		rre := regexp.MustCompile(`Apply complete! Resources: (\d+) added, (\d+) changed, (\d+) destroyed.`)
		rs := rre.FindAllStringSubmatch(line, -1)
		if len(rs) == 1 && len(rs[0]) == 4 {
			// TODO errors ignored
			r.TotalAdded, _ = strconv.Atoi(rs[0][1])
			r.TotalChanged, _ = strconv.Atoi(rs[0][2])
			r.TotalDestroyed, _ = strconv.Atoi(rs[0][3])
			return &r
		}
	}

	return nil
}

func normalizeAction(s string) string {
	if strings.HasSuffix(s, "...") {
		s = s[:len(s)-3]
	}
	return strings.ToLower(s)
}

// Output implements Terraformer.
func (t *Terraform) Output(dir string) (map[string]interface{}, error) {
	o, _, err := exe.Run(t.Log, &exe.Opt{Dir: dir}, "", "terraform", "output", "-json", "-no-color")
	if err != nil {
		return nil, err
	}
	m := map[string]interface{}{}
	err = json.Unmarshal([]byte(o), &m)
	return m, err
}
