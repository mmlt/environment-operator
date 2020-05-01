package exe

import (
	"github.com/go-logr/stdr"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRun(t *testing.T) {
	var tests = []struct {
		it string
		options    *Opt
		cmd        string
		args       []string
		in         string
		wantErr    string
		wantStdout string
		wantStderr string
	}{
		{
			it: "should_echo_on_stdout",
			cmd:        "echo",
			args:       []string{"-n", "hello world"},
			wantStdout: "hello world",
		}, 	{
			it: "should_error",
			cmd:     "ls",
			args:    []string{"nonexisting"},
			wantErr: "ls [nonexisting]: exit status 2 - ls: cannot access 'nonexisting': No such file or directory\n",
			wantStderr: "ls: cannot access 'nonexisting': No such file or directory\n",
		}, {
			it: "should_read_stdin_and_write_stdout",
			cmd:        "base64",
			args:       []string{"-d"},
			in:         "aGVsbG8gd29ybGQ=",
			wantStdout: "hello world",
		}, {
			it: "should_use_the_specified_environment",
			options: &Opt{
				Env: []string{"SONG=HappyHappyJoyJoy"},
			},
			cmd:        "env",
			wantStdout: "SONG=HappyHappyJoyJoy\n",
		}, {
			it: "should_execute_in_the_specified_dir",
			options: &Opt{
				Dir: "/tmp",
			},
			cmd:        "pwd",
			wantStdout: "/tmp\n",
		},
	}

	log := stdr.New(nil)

	for _, tst := range tests {
		t.Run(tst.it, func(t *testing.T) {
			stdout, stderr, err := Run(log, tst.options, tst.in, tst.cmd, tst.args...)
			if tst.wantErr != "" {
				assert.EqualError(t, err, tst.wantErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tst.wantStdout, stdout)
			assert.Equal(t, tst.wantStderr, stderr)
		})
	}
}
