package terraform

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"github.com/Jeffail/gabs/v2"
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
	Init(ctx context.Context, env []string, dir string) *TFResult
	// Plan creates an execution plan for the terraform configuration files in dir.
	Plan(ctx context.Context, env []string, dir string) *TFResult
	// StartApply applies the plan in dir without waiting for completion.
	// If a Cmd is returned cmd.Wait() should be called wait for completion and clean-up.
	StartApply(ctx context.Context, env []string, dir string) (*exec.Cmd, chan TFApplyResult, error)
	// StartDestroy destroys the resources specified in the plan in dir without waiting for completion.
	// If a Cmd is returned cmd.Wait() should be called wait for completion and clean-up.
	StartDestroy(ctx context.Context, env []string, dir string) (*exec.Cmd, chan TFApplyResult, error)
	// Output gets terraform output values an returns them as a map of types and values.
	// When outputs.tf contains output "xyz" { value = 7 } the returned map contains ["yxz"]["value"] == 7
	Output(ctx context.Context, env []string, dir string) (map[string]interface{}, error)
	// GetPlan reads an existing plan and returns a json structure.
	GetPlan(ctx context.Context, env []string, dir string) (*gabs.Container, error)
}

// TFResults is the output of a terraform command.
type TFResult struct {
	// Info > 0 and Errors == 0 means the command is successful.
	Info int
	// Warnings > 0 means command successful but something unexpected happened.
	Warnings int

	// Errors is a list of error messages.
	// len > 0 means the command failed.
	Errors []string

	PlanAdded   int
	PlanChanged int
	PlanDeleted int

	// Text is the verbatim output of the command.
	Text string
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
	// *ing means in-progress, *tion means completed.
	Action string
	// Most recently logged elapsed time reported by terraform.
	Elapsed string

	// Text is the verbatim output of the command.
	// NB every TFApplyResult instance holds a string with all lines known at that time.
	Text string
}

// Terraform provisions infrastructure using terraform cli.
type Terraform struct{}

var _ Terraformer = &Terraform{}

// PlanName is the name of the terraform plan.
const planName = "newplan"

// Init resolves (downloads) dependencies for the terraform configuration files in dir.
func (t *Terraform) Init(ctx context.Context, env []string, dir string) *TFResult {
	log := logr.FromContext(ctx).WithName("TFInit")

	o, _, err := exe.Run(log, &exe.Opt{Dir: dir, Env: env}, "", "terraform", "init", "-input=false", "-no-color")

	return parseInitResponse(o, err)
}

func parseInitResponse(text string, err error) *TFResult {
	r := &TFResult{
		Text: text,
	}

	if err != nil {
		r.Errors = append(r.Errors, err.Error())
	}

	ire := regexp.MustCompile("Terraform has been successfully initialized!")
	r.Info = len(ire.FindAllStringIndex(text, -1))
	wre := regexp.MustCompile("\nWarning: ")
	r.Warnings = len(wre.FindAllStringIndex(text, -1))
	// errors are detected via exit code instead of:
	//  ere := regexp.MustCompile(" errors |Terraform initialized in an empty directory!")
	//  r.Errors = append(r.Errors, ere.FindAllStringIndex(text, -1))

	return r
}

// Plan creates an execution plan for the terraform configuration files in dir.
func (t *Terraform) Plan(ctx context.Context, env []string, dir string) *TFResult {
	log := logr.FromContext(ctx).WithName("TFPlan")

	o, _, err := exe.Run(log, &exe.Opt{Dir: dir, Env: env}, "", "terraform", "plan",
		"-out="+planName, "-detailed-exitcode", "-input=false", "-no-color")
	return parsePlanResponse(o, err)
}

// ParsePlanResponse parses terraform stdout text and err and returns tfresult.
// Terraform should be run with flag '-detailed-exitcode' so it returns:
//	0 = Succeeded with empty diff (no changes)
//  1 = Error
//  2 = Succeeded with non-empty diff (changes present)
func parsePlanResponse(text string, err error) *TFResult {
	r := &TFResult{
		Text: text,
	}

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
	// errors are detected via exit code instead of:
	//  ere := regexp.MustCompile("\nError: ")
	//  r.Errors = len(ere.FindAllStringIndex(text, -1))

	// terraform returns exitcode:
	//	0 = Succeeded with empty diff (no changes)
	//  1 = Error
	//  2 = Succeeded with non-empty diff (changes present)
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == 1 {
		r.Info = 0
		r.Errors = append(r.Errors, err.Error())
	}

	return r
}

// StartApply applies the plan in dir without waiting for completion.
// If a Cmd is returned cmd.Wait() should be called wait for completion and clean-up.
func (t *Terraform) StartApply(ctx context.Context, env []string, dir string) (*exec.Cmd, chan TFApplyResult, error) {
	log := logr.FromContext(ctx).WithName("TFApply")
	ctx = logr.NewContext(ctx, log)

	cmd := exe.RunAsync(ctx, log, &exe.Opt{Dir: dir, Env: env}, "", "terraform", "apply",
		"-auto-approve", "-input=false", "-no-color", planName)

	o, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}

	cmd.Stderr = cmd.Stdout // combine

	err = cmd.Start()
	if err != nil {
		return nil, nil, err
	}

	ch := t.parseAsyncApplyResponse(log, o)

	return cmd, ch, nil
}

// StartDestroy destroys the resources specified in the plan in dir without waiting for completion.
// If a Cmd is returned cmd.Wait() should be called wait for completion and clean-up.
func (t *Terraform) StartDestroy(ctx context.Context, env []string, dir string) (*exec.Cmd, chan TFApplyResult, error) {
	log := logr.FromContext(ctx).WithName("TFDestroy")
	ctx = logr.NewContext(ctx, log)

	cmd := exe.RunAsync(ctx, log, &exe.Opt{Dir: dir, Env: env}, "", "terraform", "destroy",
		"-auto-approve", "-no-color")

	o, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}

	cmd.Stderr = cmd.Stdout // combine

	err = cmd.Start()
	if err != nil {
		return nil, nil, err
	}

	ch := t.parseAsyncApplyResponse(log, o)

	return cmd, ch, nil
}

// ParseAsyncApplyResponse parses in and returns results when interesting input is encountered.
// Close in to release the go func.
func (t *Terraform) parseAsyncApplyResponse(log logr.Logger, in io.ReadCloser) chan TFApplyResult {
	out := make(chan TFApplyResult)

	// hold running totals.
	result := &TFApplyResult{}

	go func() {
		sc := bufio.NewScanner(in)
		for sc.Scan() {
			s := sc.Text()
			log.V(3).Info("RunAsync-result", "text", s)
			r := parseApplyResponseLine(result, s)
			if r != nil {
				out <- *r
			}
		}
		if err := sc.Err(); err != nil {
			log.Error(err, "parseAsyncApplyResponse")
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

	r.Text = r.Text + line

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
			// errors are ignored
			r.TotalAdded, _ = strconv.Atoi(rs[0][1])
			r.TotalChanged, _ = strconv.Atoi(rs[0][2])
			r.TotalDestroyed, _ = strconv.Atoi(rs[0][3])
			return &r
		}
	}

	if ss[0] == "Destroy" {
		rre := regexp.MustCompile(`Destroy complete! Resources: (\d+) destroyed.`)
		rs := rre.FindAllStringSubmatch(line, -1)
		if len(rs) == 1 && len(rs[0]) == 2 {
			// errors are ignored
			r.TotalDestroyed, _ = strconv.Atoi(rs[0][1])
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

// Output gets terraform output values an returns them as a map of types and values.
// When outputs.tf contains output "xyz" { value = 7 } the returned map contains ["yxz"]["value"] == 7
func (t *Terraform) Output(ctx context.Context, env []string, dir string) (map[string]interface{}, error) {
	log := logr.FromContext(ctx).WithName("TFOutput")

	o, _, err := exe.Run(log, &exe.Opt{Dir: dir, Env: env}, "", "terraform", "output", "-json", "-no-color")
	if err != nil {
		return nil, err
	}
	m := map[string]interface{}{}
	err = json.Unmarshal([]byte(o), &m)
	return m, err
}
