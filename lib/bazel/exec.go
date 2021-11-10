package bazel

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
)

// Command wraps an exec.Cmd, providing a common way to access stdout and stderr
// from the underlying command across production code and tests.
type Command interface {
	// Run runs the underlying exec.Cmd.
	Run() error

	// Close releases resources associated with this command. It should be called
	// once the output is no longer needed/has already been consumed.
	Close() error

	// Stdout provides access to the command's stdout. Run() must be called first.
	// The caller should close the returned io.ReadCloser.
	Stdout() (io.ReadCloser, error)

	// Stderr provides access to the command's stderr. Run() must be called first.
	// The caller should close the returned io.ReadCloser.
	Stderr() (io.ReadCloser, error)

	// StdoutContents reads all of stdout into a raw byte slice.
	StdoutContents() ([]byte, error)

	// StderrContents reads stderr into a string. This method cannot fail, as it
	// is intended to be used only in logging/errors; if reading fails, a sentinel
	// error string will be returned instead.
	StderrContents() string
}

// fileCommand buffers stdout and stderr from the underlying command to a
// temporary file.
type fileCommand struct {
	cmd           *exec.Cmd
	stdoutPath    string
	stderrPath    string
	cargoHomePath string
}

// NewCommand returns a fileCommand wrapping the provided exec.Cmd. It is
// defined as a var so it can be stubbed in unit tests.
var NewCommand = func(cmd *exec.Cmd) (Command, error) {
	stdout, err := ioutil.TempFile("", "bazel_stdout_*")
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout file: %w", err)
	}
	stdout.Close()
	stderr, err := ioutil.TempFile("", "bazel_stderr_*")
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr file: %w", err)
	}
	stderr.Close()

	// BUG(INFRA-140) - By default, cargo will download packages to a well-known
	// directory under $HOME; this will mean that parallel bazel invocations could
	// race on this directory if they both fetch Cargo packages. Cargo respects
	// the $CARGO_HOME environment variable, so set it to something unique for
	// this invocation.
	env := os.Environ()
	cargoHome, err := ioutil.TempDir("", "bazel_cargo_home_*")
	if err != nil {
		return nil, fmt.Errorf("failed to create tmpdir for $CARGO_HOME: %w", err)
	}
	env = append(env, "CARGO_HOME="+cargoHome)
	cmd.Env = env

	return &fileCommand{
		cmd:           cmd,
		stdoutPath:    stdout.Name(),
		stderrPath:    stderr.Name(),
		cargoHomePath: cargoHome,
	}, nil
}

// Run runs the command, outputting stdout and stderr to temporary files.
func (c *fileCommand) Run() error {
	stdout, err := os.OpenFile(c.stdoutPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return fmt.Errorf("failed to connect command to stdout file: %w", err)
	}
	defer stdout.Close()

	stderr, err := os.OpenFile(c.stderrPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return fmt.Errorf("failed to connect command to stderr file: %w", err)
	}
	defer stderr.Close()

	c.cmd.Stdout = stdout
	c.cmd.Stderr = stderr
	return c.cmd.Run()
}

// Stdout opens and returns the temporary file containing stdout contents.
func (c *fileCommand) Stdout() (io.ReadCloser, error) {
	f, err := os.Open(c.stdoutPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open stdout file: %w", err)
	}
	return f, nil
}

// Stderr opens and returns the temporary file containing stderr contents.
func (c *fileCommand) Stderr() (io.ReadCloser, error) {
	f, err := os.Open(c.stderrPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open stderr file: %w", err)
	}
	return f, nil
}

// StdoutContents returns the contents of the stdout temporary file.
func (c *fileCommand) StdoutContents() ([]byte, error) {
	contents, err := ioutil.ReadFile(c.stdoutPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read stdout file: %w", err)
	}
	return contents, nil
}

// StderrContents returns the contents of the stderr temporary file. It is
// assumed that this is a text file, and the contents have space trimmed before
// returning.
func (c *fileCommand) StderrContents() string {
	contents, err := ioutil.ReadFile(c.stderrPath)
	if err != nil {
		return "<failed to read stderr contents>"
	}
	return string(bytes.TrimSpace(contents))
}

// Close removes stdout and stderr temporary files.
func (c *fileCommand) Close() error {
	os.Remove(c.stdoutPath)
	os.Remove(c.stderrPath)
	os.RemoveAll(c.cargoHomePath)
	return nil
}
