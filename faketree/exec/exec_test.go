package exec

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRun(t *testing.T) {
	ctx := context.Background()
	faketreeBin = "/opt/enfabrica/bin/faketree"

	gotErr := Run(
		ctx,
		"$",
		map[string]string{
			os.Getenv("TEST_SRCDIR"): os.Getenv("TEST_TMPDIR"),
		},
		os.Getenv("TEST_TMPDIR"),
		[]string{"/bin/true"},
	)
	assert.Nilf(t, gotErr, "got error: %v; want no error", gotErr)

	gotErr = Run(
		ctx,
		"$",
		map[string]string{
			os.Getenv("TEST_SRCDIR"): os.Getenv("TEST_TMPDIR"),
		},
		os.Getenv("TEST_TMPDIR"),
		[]string{"/bin/false"},
	)
	assert.NotNil(t, gotErr)
}
