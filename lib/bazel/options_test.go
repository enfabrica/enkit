package bazel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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