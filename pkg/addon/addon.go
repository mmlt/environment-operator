// Package addon deploys Kubernetes resources to a cluster.
package addon

import (
	"bufio"
	"context"
	"github.com/go-logr/logr"
	"github.com/mmlt/environment-operator/pkg/util/exe"
	"io"
	"os/exec"
	"strings"
	"text/scanner"
)

// Addonr is able to provision Kubernetes resources.
type Addonr interface {
	// Start runs kubectl-tmplt concurrently in dir and returns a cmd and a channel of KTResults.
	// The jobPath refers to a yaml file with resources to apply.
	// The valuesPath refers to yaml file with values that parameterize the job resources.
	// The kubeconfigPath refers to a kube config file with current-context refering to the target cluster.
	// The channel will be closed when kubectl-tmplt exits.
	// cmd.Wait() must be called to clean-up.
	Start(ctx context.Context, dir, jobPath, valuesPath, kubeconfigPath string) (*exec.Cmd, chan KTResult, error)
}

// KTResult
type KTResult struct {
	// Running count of the number of resources that have been created, updated and deleted.
	Added, Changed, Deleted int

	// Errors is a list of error messages.
	Errors []string

	// Most recently logged kubernetes resource name.
	Object string
	// The sequence number of the object.
	ObjectID string
	// Most recently logged action being performed; creating (creation), modifying (modification), destroying (destruction).
	// *ing means in-progress, *tion means completed. TODO consider normalizing *tion to *ed
	Action string
}

// Addon provisions Kubernetes resources using kubectl-tmplt cli.
type Addon struct {
	Log logr.Logger
}

// Start implements Addonr.
func (a *Addon) Start(ctx context.Context, dir, jobPath, valuesPath, kubeconfigPath string) (*exec.Cmd, chan KTResult, error) {
	cmd := exe.RunAsync(ctx, a.Log, &exe.Opt{Dir: dir}, "", "kubectl-tmplt",
		"-m", "apply",
		"--job-file", jobPath,
		"--set-file", valuesPath,
		"--kubeconfig", kubeconfigPath)

	o, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}

	cmd.Stderr = cmd.Stdout // combine

	err = cmd.Start()
	if err != nil {
		return nil, nil, err
	}

	ch := a.parseAsyncAddonResponse(o)

	return cmd, ch, nil
}

// ParseAsyncAddonResponse parses in and returns results when interesting input is encountered.
// Close in to release the go func.
func (a *Addon) parseAsyncAddonResponse(in io.ReadCloser) chan KTResult {
	out := make(chan KTResult)

	// hold running totals.
	result := &KTResult{}

	go func() {
		sc := bufio.NewScanner(in)
		for sc.Scan() {
			s := sc.Text()
			a.Log.V(3).Info("RunAsync-result", "text", s)
			r := parseAddonResponseLine(result, s)
			if r != nil {
				out <- *r
			}
		}
		if err := sc.Err(); err != nil {
			a.Log.Error(err, "parseAsyncAddonResponse")
		}

		close(out)
	}()

	return out
}

// ParseAddonResponseLine parses a line.
// If content can be extracted from line it returns an updated shallow copy of in, otherwise it returns nil.
// It increments in running counters.
func parseAddonResponseLine(in *KTResult, line string) *KTResult {
	if len(line) < 3 {
		// not interesting.
		return nil
	}

	r := *in

	if line[0] == 'E' {
		r.Errors = append(r.Errors, line[2:])
		return &r
	}

	if line[0] != 'I' {
		return nil
	}

	var last3 []string
	var sc scanner.Scanner
	sc.Init(strings.NewReader(line))
	for tok := sc.Scan(); tok != scanner.EOF; tok = sc.Scan() {
		last3 = append(last3, strings.Trim(sc.TokenText(), "\""))
		if len(last3) > 3 {
			last3 = last3[1:]
		}
		if len(last3) == 3 && last3[1] == "=" {
			switch last3[0] {
			case "txt":
				r.Object = last3[2]
			case "msg":
				r.Action = last3[2]
			case "id":
				r.ObjectID = last3[2]
				// not used: "tpl" "level"
			}
		}
	}

	return &r
}