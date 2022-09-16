package bazel

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
)

type fakeCommand struct {
	runErr         error
	closeErr       error
	stdout         io.ReadCloser
	stdoutContents []byte
	stdoutErr      error
	stderr         io.ReadCloser
	stderrErr      error
}

func (c *fakeCommand) Run() error {
	return c.runErr
}

func (c *fakeCommand) Close() error {
	return c.closeErr
}

func (c *fakeCommand) String() string {
	return "your-fakest-command"
}

func (c *fakeCommand) Stdout() (io.ReadCloser, error) {
	if c.stdoutErr != nil {
		return nil, c.stdoutErr
	}
	return c.stdout, nil
}

func (c *fakeCommand) Stderr() (io.ReadCloser, error) {
	if c.stderrErr != nil {
		return nil, c.stderrErr
	}
	return c.stderr, nil
}

func (c *fakeCommand) StdoutContents() ([]byte, error) {
	contents, err := ioutil.ReadAll(c.stdout)
	if err != nil {
		return nil, fmt.Errorf("failed to read stdout file: %w", err)
	}
	return contents, nil
}

func (c *fakeCommand) StderrContents() string {
	contents, err := ioutil.ReadAll(c.stderr)
	if err != nil {
		return "<failed to read stderr contents>"
	}
	return string(bytes.TrimSpace(contents))
}
