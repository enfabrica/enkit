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

// Raise an error on non-zero exit code.
func (result *RunResult) CheckExitCode() error {
	if result.exit_code != 0 {
		return fmt.Errorf("Command failed (rc=%d): %q", result.exit_code, result.command)
	}
	return nil
}

// Fail and terminate on non-zero exit code.
// Example: result := Run(...).MustSucceed()
func (result *RunResult) MustSucceed() *RunResult {
	if result.exit_code != 0 {
		Logger().Fatalf("Command failed (rc=%d): %q", result.exit_code, result.command)
	}
	return result
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

// Cmd specifies a command to run.  Similar to exec.Cmd, but simpler
// for the funcitonality gee needs.
type Cmd struct {
	Args        []string // the argv list to execute
	Dir         string   // if non-empty, run in this directory
	Quiet       bool     // don't print stdout to console
	VeryQuiet   bool     // don't print stdout or stderr to console
	Interactive bool     // run subprocess interactively with the user.
	FromFile    string   // read stdin from file, if specified.
	CanFail     bool     // don't terminate if exit code is non-zero.
}

// A wrapper around exec.Cmd with a slightly simplified interface.
//
// A little syntactic sugar: The initial Cmd object could specify the entire
// command.  Or, the cmd Object could simply specify execution options (or
// "nil" for defaults), and the command arguments can be specified by the
// variadic list of strings in the command.
func (runner *singletonRunner) Run(a Cmd, args ...string) *RunResult {
	a.Args = append(a.Args, args...)
	result := &RunResult{}
	result.command = a.Args
	cmd := exec.Command(a.Args[0])
	cmd.Args = a.Args
	cmd.Dir = a.Dir
	cmd.Env = runner.env
	if a.Interactive {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
	} else if a.VeryQuiet {
		cmd.Stdout = &result.stdout
		cmd.Stderr = &result.stderr
		cmd.Stdin = nil
	} else if a.Quiet {
		cmd.Stdout = &result.stdout
		cmd.Stderr = io.MultiWriter(&result.stderr, os.Stderr)
		cmd.Stdin = nil
	} else {
		cmd.Stdout = io.MultiWriter(&result.stdout, os.Stdout)
		cmd.Stderr = io.MultiWriter(&result.stderr, os.Stderr)
		cmd.Stdin = nil
	}
	if a.FromFile != "" {
		file, err := os.Open(a.FromFile)
		if err != nil {
			Logger().Fatalf("Could not open %q for reading: %q", a.FromFile, err)
		}
		cmd.Stdin = file
	}
	Logger().Command(cmd.Args...)
	err := cmd.Run()
	if err != nil {
		Logger().Error(err.Error())
		panic(err)
	}
	result.exit_code = cmd.ProcessState.ExitCode()
	Logger().Debugf("Exited with exit_code=%d", result.exit_code)
	if !a.CanFail {
		result = result.MustSucceed()
	}
	return result
}

func (runner *singletonRunner) RunGit(a Cmd, args ...string) *RunResult {
	a.Args = append([]string{viper.GetString("git_path")}, a.Args...)
	a.Args = append(a.Args, args...)
	return runner.Run(a)
}

func (runner *singletonRunner) RunGh(a Cmd, args ...string) *RunResult {
	a.Args = append([]string{viper.GetString("gh_path")}, a.Args...)
	a.Args = append(a.Args, args...)
	return runner.Run(a)
}
