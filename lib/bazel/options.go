package bazel

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	
	"github.com/enfabrica/enkit/lib/multierror"
)

// subcommand is implemented by an arguments struct for each bazel subcommand
// (such as 'query', 'build', etc.).
type subcommand interface {
	// Args returns the subcommand plus all the subcommands' arguments.
	Args() []string
}

// baseOptions captures supported bazel startup flags.
type baseOptions struct {
	// Bazel's cache directory for this workspace.
	OutputBase string
}

// Option modifies Bazel startup options.
type BaseOption func(*baseOptions)

type BaseOptions []BaseOption

// WithOutputBase sets --output_base for this bazel invocation.
func WithOutputBase(outputBase string) BaseOption {
	return func(o *baseOptions) {
		o.OutputBase = outputBase
	}
}

// flags returns the startup flags as passed to bazel.
func (o *baseOptions) flags() []string {
	var f []string
	if o.OutputBase != "" {
		f = append(f, "--output_base", o.OutputBase)
	}
	return f
}

// apply applies all the options to this option struct.
func (opts BaseOptions) apply(o *baseOptions) {
	for _, opt := range opts {
		opt(o)
	}
}

// queryOptions holds all the supported arguments for `bazel query` invocations.
type queryOptions struct {
	query string

	keepGoing       bool
	unorderedOutput bool
	workspaceLog *os.File
}

// Args returns the `query` and relevant subcommand arguments as passed to bazel.
func (o *queryOptions) Args() []string {
	f := []string{"query", "--output=streamed_proto"}
	if o.keepGoing {
		f = append(f, "--keep_going")
	}
	if o.unorderedOutput {
		f = append(f, "--order_output=no")
	}
	if o.workspaceLog != nil {
		// See https://github.com/bazelbuild/bazel/issues/6807 for tracking issue
		// making this flag non-experimental
		f = append(f, "--experimental_workspace_rules_log_file", o.workspaceLog.Name())
	}
	f = append(f, "--", o.query)
	return f
}

// filterError filters out expected error codes based on the provided query
// arguments.
func (o *queryOptions) filterError(err error) error {
	if err == nil || !o.keepGoing {
		return nil
	}

	var execErr *exec.ExitError
	if errors.As(err, &execErr) {
		// PARTIAL_ANALYSIS_FAILURE is expected when --keep_going is passed
		// https://github.com/bazelbuild/bazel/blob/86409b7a248d1cb966268451f9aa4db0763c3eb2/src/main/java/com/google/devtools/build/lib/util/ExitCode.java#L38
		if execErr.ExitCode() == 3 {
			return nil
		}
	}

	return err
}

func (o *queryOptions) Close() error {
	var errs []error
	if o.workspaceLog != nil {
		errs = append(errs, o.workspaceLog.Close())
		errs = append(errs, os.RemoveAll(o.workspaceLog.Name()))
	}
	return multierror.New(errs)
}

// QueryOption modifies bazel query subcommand flags.
type QueryOption func(*queryOptions)

type QueryOptions []QueryOption

// WithKeepGoing sets `--keep_going` for this `bazel query` invocation.
func WithKeepGoing() QueryOption {
	return func(o *queryOptions) {
		o.keepGoing = true
	}
}

// WithUnorderedOutput sets `--order_output=no` for this `bazel query` invocation.
func WithUnorderedOutput() QueryOption {
	return func(o *queryOptions) {
		o.unorderedOutput = true
	}
}

func WithTempWorkspaceRulesLog() (QueryOption, error) {
	f, err := ioutil.TempFile("", "bazel_workspace_log_*.pb")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp workspace log file: %w", err)
	}

	return func(o *queryOptions) {
		o.workspaceLog = f
	}, nil
}

// apply applies all the options to this option struct.
func (opts QueryOptions) apply(o *queryOptions) {
	for _, opt := range opts {
		opt(o)
	}
}
