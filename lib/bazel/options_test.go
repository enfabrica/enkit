package bazel

import (
	"errors"
	"fmt"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

// exitError manufactures a *exec.ExitError with a specified return code.
func exitError(retcode int) error{
	// exec.ExitError wraps os.ProcessState which is opaque, so there's no way to
	// construct an error via the normal means.
	// Instead, exec a script which exits with the specified code, and then return
	// the error that the stdlib generated for us.
	_, err := exec.Command("bash", "-c", fmt.Sprintf("exit %d", retcode)).Output()
	return err
}

func TestQueryOptionsArgs(t *testing.T) {
	tempLog, err := WithTempWorkspaceRulesLog()
	assert.NoError(t, err)
	opts := QueryOptions{
		WithKeepGoing(),
		WithUnorderedOutput(),
		tempLog,
	}
	opt := &queryOptions{}
	opts.apply(opt)
	got := opt.Args()

	assert.Contains(t, got, "query")
	assert.Contains(t, got, "--output=streamed_proto")
	assert.Contains(t, got, "--order_output=no")
	assert.Contains(t, got, "--keep_going")
	assert.Contains(t, got, "--experimental_workspace_rules_log_file")
}

func TestQueryOptionsFilterError(t *testing.T) {
	testCases := []struct {
		desc string
		options *queryOptions
		err error
		wantErr bool
	} {
		{
			desc: "default propagates all errors",
			options: &queryOptions{},
			err: errors.New("some error"),
			wantErr: true,
		},
		{
			desc: "keep_going ignores exit code 3",
			options: &queryOptions{keepGoing: true},
			err: exitError(3),
			wantErr: false,
		},
		{
			desc: "keep_going propagates exit code 4",
			options: &queryOptions{keepGoing: true},
			err: exitError(4),
			wantErr: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func (t *testing.T) {
			gotErr := tc.options.filterError(tc.err)

			assert.Equalf(t, gotErr != nil, tc.wantErr, "gotErr was '%v' but wantErr was %v", gotErr, tc.wantErr)
		})
	}
}