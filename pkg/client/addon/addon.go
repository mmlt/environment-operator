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
	"unicode"
)

// Addonr is able to provision Kubernetes resources.
type Addonr interface {
	// Start runs kubectl-tmplt concurrently in dir and returns a cmd and a channel of KTResults.
	// The jobPath refers to a yaml file with resources to apply.
	// The valuesPath refers to yaml file with values that parameterize the job resources.
	// The kubeconfigPath refers to a kube config file with current-context referring to the target cluster.
	// The masterVaultPath refers to a directory containing the config of the Vault to use for {{ vault }}.
	// The channel will be closed when kubectl-tmplt exits.
	// cmd.Wait() must be called to clean-up.
	Start(ctx context.Context, env []string, dir, jobPath, valuesPath, kubeconfigPath, masterVaultPath string) (*exec.Cmd, chan KTResult, error)
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
	// *ing means in-progress, *tion means completed.
	Action string
}

// Addon provisions Kubernetes resources using kubectl-tmplt cli.
type Addon struct {
}

// Start implements Addonr.
func (a *Addon) Start(ctx context.Context, env []string, dir, jobPath, valuesPath, kubeconfigPath, masterVaultPath string) (*exec.Cmd, chan KTResult, error) {
	log := logr.FromContext(ctx).WithName("Addon")
	ctx = logr.NewContext(ctx, log)

	cmd := exe.RunAsync(ctx, log, &exe.Opt{Dir: dir, Env: env}, "", "kubectl-tmplt",
		"-m", "apply-with-actions",
		"--job-file", jobPath,
		"--set-file", valuesPath,
		"--kubeconfig", kubeconfigPath,
		"--master-vault-path", masterVaultPath)

	o, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}

	cmd.Stderr = cmd.Stdout // combine

	err = cmd.Start()
	if err != nil {
		return nil, nil, err
	}

	ch := a.parseAsyncAddonResponse(log, o)

	return cmd, ch, nil
}

// ParseAsyncAddonResponse parses in and returns results when interesting input is encountered.
// Close in to release the go func.
func (a *Addon) parseAsyncAddonResponse(log logr.Logger, in io.ReadCloser) chan KTResult {
	out := make(chan KTResult)

	// hold running totals.
	changed := 0

	go func() {
		sc := bufio.NewScanner(in)
		for sc.Scan() {
			s := sc.Text()
			log.V(3).Info("RunAsync-result", "text", s)
			r := parseAddonResponseLine(s)
			if r != nil {
				// every line counts as a change.
				changed++
				r.Changed = changed
				out <- *r
			}
		}
		if err := sc.Err(); err != nil {
			log.Error(err, "parseAsyncAddonResponse")
		}

		close(out)
	}()

	return out
}

// ParseAddonResponseLine parses a line and returns the result.
// If parsing fails it returns nil.
func parseAddonResponseLine(line string) *KTResult {
	if len(line) < 3 {
		// not interesting.
		return nil
	}

	r := KTResult{}

	if line[0] == 'E' {
		r.Errors = append(r.Errors, line[2:])
		return &r
	}

	if line[0] != 'I' {
		return nil
	}

	kvDo(line, func(k, v string) {
		switch k {
		case "txt":
			r.Object = v
		case "msg":
			r.Action = v
		case "id":
			r.ObjectID = v
			// case "tpl" "level" are ignored.
		}
	})

	return &r
}

// KVDo calls do for each k=v pattern in line.
// Quoted strings are allowed for k and v.
func kvDo(line string, do func(k, v string)) {
	lastQuote := rune(0)
	ss := strings.FieldsFunc(line, func(c rune) bool {
		switch {
		case c == lastQuote:
			lastQuote = rune(0)
			return false
		case lastQuote != rune(0):
			return false
		case unicode.In(c, unicode.Quotation_Mark):
			lastQuote = c
			return false
		default:
			return unicode.IsSpace(c)
		}
	})
	for _, s := range ss {
		kv := strings.Split(s, "=")
		if len(kv) != 2 {
			continue
		}
		k := strings.Trim(kv[0], "\"")
		v := strings.Trim(kv[1], "\"")
		do(k, v)
	}
}
