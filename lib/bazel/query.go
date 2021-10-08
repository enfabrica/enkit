package bazel

import (
	"fmt"
	"io"
	"os/exec"

	bpb "github.com/enfabrica/enkit/lib/bazel/proto"
	"github.com/enfabrica/enkit/lib/proto/delimited"

	"google.golang.org/protobuf/proto"
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
			errChan <- err // Don't wrap, so raw ExitError can be picked up by the caller
		}
	}()

	return pipeReader, errChan, nil
}

// Query performs a `bazel query` using the provided query string. If
// `keep_going` is set, then `--keep_going` is set on the bazel commandline, and
// errors from the bazel process are ignored.
func (w *Workspace) Query(query string, options ...QueryOption) (map[string]*bpb.Target, error) {
	queryOpts := (&queryOptions{query: query}).apply(options)

	cmd := w.bazelCommand(queryOpts)
	resultStream, errChan, err := streamedBazelCommand(cmd)
	if err != nil {
		return nil, err
	}

	targets := map[string]*bpb.Target{}
	var readErr error
	readDoneChan := make(chan struct{})

	go func() {
		defer close(readDoneChan)
		rdr := delimited.NewReader(resultStream)
		var buf []byte
		var err error
		for buf, err = rdr.Next(); err == nil; buf, err = rdr.Next() {
			var target bpb.Target
			if err := proto.Unmarshal(buf, &target); err != nil {
				readErr = err
				return
			}
			targets[targetName(&target)] = &target
		}
		if err != io.EOF {
			readErr = err
		}
	}()
	<-readDoneChan

	if readErr != nil {
		return nil, readErr
	}

	if err := queryOpts.filterError(<-errChan); err != nil {
		return nil, err
	}

	return targets, nil
}

// targetName returns the name of a Target message, which is part of a
// pseudo-union message (enum + one populated optional field).
func targetName(t *bpb.Target) string {
	switch t.GetType() {
	case bpb.Target_RULE:
		return t.GetRule().GetName()
	case bpb.Target_SOURCE_FILE:
		return t.GetSourceFile().GetName()
	case bpb.Target_GENERATED_FILE:
		return t.GetGeneratedFile().GetName()
	case bpb.Target_PACKAGE_GROUP:
		return t.GetPackageGroup().GetName()
	case bpb.Target_ENVIRONMENT_GROUP:
		return t.GetEnvironmentGroup().GetName()
	}
	// This shouldn't happen; check that all cases are covered.
	panic(fmt.Sprintf("can't get name for type %q", t.GetType()))
}
