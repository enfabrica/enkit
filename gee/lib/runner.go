package lib

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
)

type Runner struct {
	env []string
	dir string
}

var runner *Runner = nil

type RunResult struct {
	stdout    bytes.Buffer
	stderr    bytes.Buffer
	exit_code int
}

func NewRunner() *Runner {
	var err error
	runner := new(Runner)
	runner.env = os.Environ()
	runner.dir, err = os.Getwd()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return runner
}

func GetRunner() *Runner {
	if runner == nil {
		runner = NewRunner()
	}
	return runner
}

func (runner *Runner) RunInDir(path string, args []string, dir string) *RunResult {
	result := &RunResult{}
	stdout_writer := io.MultiWriter(&result.stdout, os.Stdout)
	stderr_writer := io.MultiWriter(&result.stderr, os.Stderr)
	GetLogger().Info("foo", "bar")
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
