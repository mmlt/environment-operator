package exe

import (
	"bytes"
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"io"
	"os/exec"
)

// Opt are the exec options, see https://godoc.org/os/exec#Cmd for details.
type Opt struct {
	// RepoDir is the working directory.
	Dir string
	// Env is the execution environment.
	Env []string
}

// Run executes 'cmd' with 'stdin', 'args' and (optional) 'options'.
// Return stdout and stderr upon completion.
func Run(log logr.Logger, options *Opt, stdin string, cmd string, args ...string) (stdout, stderr string, err error) {
	log.V(2).Info("Run", "cmd", cmd, "args", args)

	c := exec.Command(cmd, args...)

	if options != nil {
		c.Env = options.Env
		c.Dir = options.Dir
	}

	if stdin != "" {
		sin, err := c.StdinPipe()
		if err != nil {
			log.Error(err, "should not happen")
			return "", "", err
		}

		go func() {
			_, err := io.WriteString(sin, stdin)
			if err != nil {
				log.Error(err, "write stdin")
			}
			err = sin.Close()
			if err != nil {
				log.Error(err, "close stdin")
			}
		}()
	}

	var sout, serr bytes.Buffer
	c.Stdout, c.Stderr = &sout, &serr
	err = c.Run()
	stdout, stderr = string(sout.Bytes()), string(serr.Bytes())
	log.V(3).Info("Run-result", "stderr", stderr, "stdout", stdout)
	if err != nil {
		return stdout, stderr, fmt.Errorf("%s %v: %w - %s", cmd, args, err, stderr)
	}

	return
}

// RunAsync returns an exec.Cmd that requires Start() and Wait() to run it.
func RunAsync(ctx context.Context, log logr.Logger, options *Opt, _ /*stdin*/ string, cmd string, args ...string) *exec.Cmd {
	log.V(2).Info("RunAsync", "cmd", cmd, "args", args)

	c := exec.CommandContext(ctx, cmd, args...)

	if options != nil {
		c.Env = options.Env
		c.Dir = options.Dir
	}

	return c
}
