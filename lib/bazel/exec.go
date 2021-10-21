package bazel

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// streamedBazelCommand exec's out to bazel in the specified workspace with the
// specified arguments. It is a variable that allows for stubbing during tests.
//
// In production, the implementation will return an io.Reader containing stdout,
// an error channel that will emit any errors (if present) during execution, and
// an error if any occur while starting the command. errChan is closed after the
// command completes, but the caller should read all of the returned io.Reader
// before checking the error channel.
var streamedBazelCommand = func(cmd *exec.Cmd) (io.Reader, chan error, error) {
	// Uncomment this line to debug bazel issues using its stderr output.
	// TODO(scott): Log this somehow
	//cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("can't get stdout for bazel query: %w", err)
	}
	err = cmd.Start()
	if err != nil {
		return nil, nil, fmt.Errorf("can't start bazel command: %w", err)
	}

	pipeReader, pipeWriter := io.Pipe()
	errChan := make(chan error)
	go func() {
		defer close(errChan)
		_, err := io.Copy(pipeWriter, stdout)
		pipeWriter.Close()
		if err != nil {
			errChan <- fmt.Errorf("while copying stdout from bazel command: %w", err)
		}
		err = cmd.Wait()
		if err != nil {
			errChan <- fmt.Errorf("command failed: `%s`: %w", strings.Join(cmd.Args, " "), err)
		}
	}()

	return pipeReader, errChan, nil
}

// runBazelCommand runs the prepared command and returns a buffer containing
// stdout. It is declared as a var to allow for easy stubbing in unit tests.
var runBazelCommand = func(cmd *exec.Cmd) (string, error) {
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(bytes.TrimSpace(out)), nil
}
