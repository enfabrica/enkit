package exec

import (
	"context"
	"os"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	ctx := context.Background()

	faketreeRunfilesPath, err := runfiles.Rlocation("enkit/faketree/faketree_/faketree")
	require.NoError(t, err)
	defer func(oldPath string) {
		faketreeBin = oldPath
	}(faketreeBin)
	faketreeBin = faketreeRunfilesPath

	gotErr := Run(
		ctx,
		"$",
		map[string]string{
			os.Getenv("TEST_SRCDIR"): os.Getenv("TEST_TMPDIR"),
		},
		os.Getenv("TEST_TMPDIR"),
		[]string{"/bin/true"},
	)
	assert.NoError(t, gotErr)

	gotErr = Run(
		ctx,
		"$",
		map[string]string{
			os.Getenv("TEST_SRCDIR"): os.Getenv("TEST_TMPDIR"),
		},
		os.Getenv("TEST_TMPDIR"),
		[]string{"/bin/false"},
	)
	assert.ErrorContains(t, gotErr, "exit status 1")
}
