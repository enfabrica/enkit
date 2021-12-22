package lib

import (
	"bytes"
	"fmt"
	"github.com/spf13/viper"
	"io"
	"os"
	"os/exec"
)

type singletonRunner struct {
	env []string
	dir string
}

var runner *singletonRunner = nil

// The result of executing a subcommand.
type RunResult struct {
	command   []string
	stdout    bytes.Buffer
	stderr    bytes.Buffer
	exit_code int
}

func (result *RunResult) CheckExitCode() error {
	if result.exit_code != 0 {
		return fmt.Errorf("Got RC=%d from command %q", result.exit_code, result.command)
	}
	return nil
}

func newRunner() *singletonRunner {
	var err error
	runner := new(singletonRunner)
	runner.env = os.Environ()
	runner.dir, err = os.Getwd()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return runner
}

// Return a handle to the singleton Runner object.
func Runner() *singletonRunner {
	if runner == nil {
		runner = newRunner()
	}
	return runner
}

// Execute a command in a specified directory.
func (runner *singletonRunner) RunInDir(dir string, args ...string) *RunResult {
	result := &RunResult{}
	result.command = args
	stdout_writer := io.MultiWriter(&result.stdout, os.Stdout)
	stderr_writer := io.MultiWriter(&result.stderr, os.Stderr)
	cmd := exec.Command(args[0])
	cmd.Args = args
	cmd.Dir = dir
	cmd.Env = runner.env
	cmd.Stdout = stdout_writer
	cmd.Stderr = stderr_writer
	cmd.Stdin = os.Stdin
	Logger().Command(args...)
	err := cmd.Run()
	if err != nil {
		Logger().Error(err.Error())
		panic(err)
	}
	result.exit_code = cmd.ProcessState.ExitCode()
	Logger().Debugf("Exited with exit_code=%d", result.exit_code)
	return result
}

// Execute a command in the current working directory.
func (runner *singletonRunner) Run(args ...string) *RunResult {
	return runner.RunInDir(runner.dir, args...)
}

func (runner *singletonRunner) RunGit(args ...string) *RunResult {
	cmd := append([]string{viper.GetString("git_path")}, args...)
	return runner.RunInDir(runner.dir, cmd...)
}

func (runner *singletonRunner) RunGh(args ...string) *RunResult {
	cmd := append([]string{viper.GetString("gh_path")}, args...)
	return runner.RunInDir(runner.dir, cmd...)
}
