package lib

import (
	"bytes"
	"io"
	"os"
	"os/exec"
)

type Runner struct {
	env []string
	dir string
}

type RunResult struct {
	stdout    bytes.Buffer
	stderr    bytes.Buffer
	exit_code int
}

func NewRunner() *Runner {
	runner := new(Runner)
	runner.env = os.Environ()
	return runner
}

func (runner *Runner) RunInDir(path string, args []string, dir string) *RunResult {
	result := &RunResult{}
	stdout_writer := io.MultiWriter(&result.stdout, os.Stdout)
	stderr_writer := io.MultiWriter(&result.stderr, os.Stderr)
	cmd := exec.Command(path)
	cmd.Args = args
	cmd.Dir = dir
	cmd.Env = runner.env
	cmd.Stdout = stdout_writer
	cmd.Stderr = stderr_writer
	cmd.Stdin = os.Stdin
	err := cmd.Run()
	if err != nil {
		panic(err)
	}
	result.exit_code = cmd.ProcessState.ExitCode()
	return result
}

func (runner *Runner) Run(path string, args []string) *RunResult {
	return runner.RunInDir(path, args, runner.dir)
}
