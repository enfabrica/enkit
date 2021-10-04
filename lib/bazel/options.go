package bazel

import (
	"os/exec"
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
type Option func(*baseOptions)

// WithOutputBase sets --output_base for this bazel invocation.
func WithOutputBase(outputBase string) Option {
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
func (o *baseOptions) apply(opts []Option) *baseOptions {
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// queryOptions holds all the supported arguments for `bazel query` invocations.
type queryOptions struct {
	query string

	keepGoing       bool
	unorderedOutput bool
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
	f = append(f, "--", o.query)
	return f
}

// filterError filters out expected error codes based on the provided query
// arguments.
func (o *queryOptions) filterError(err error) error {
	if err == nil {
		return nil
	}

	if o.keepGoing {
		if err, ok := err.(*exec.ExitError); ok {
			// PARTIAL_ANALYSIS_FAILURE is expected when --keep_going is passed
			// https://github.com/bazelbuild/bazel/blob/86409b7a248d1cb966268451f9aa4db0763c3eb2/src/main/java/com/google/devtools/build/lib/util/ExitCode.java#L38
			if err.ExitCode() == 3 {
				return nil
			}
		}
	}

	return err
}

// QueryOption modifies bazel query subcommand flags.
type QueryOption func(*queryOptions)

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

// apply applies all the options to this option struct.
func (o *queryOptions) apply(opts []QueryOption) *queryOptions {
	for _, opt := range opts {
		opt(o)
	}
	return o
}
