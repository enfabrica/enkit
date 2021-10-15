package bazel

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"testing"

	"github.com/enfabrica/enkit/lib/errdiff"

	rulesgo "github.com/bazelbuild/rules_go/go/tools/bazel"
	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"
)

func mustFindRunfile(path string) string {
	p, err := rulesgo.Runfile(path)
	if err != nil {
		panic(fmt.Sprintf("can't find runfile %q: %v", path, err))
	}
	return p
}

func TestQueryOutput(t *testing.T) {
	testCases := []struct {
		desc            string
		queryOutputFile string
		wantCount       int
		wantErr         string
	}{
		{
			desc:            "query deps //lib/bazel/commands/...",
			queryOutputFile: mustFindRunfile("lib/bazel/testdata/query_deps_lib_bazel_commands.pb"),
			wantCount:       1740,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			stubs := gostub.Stub(&streamedBazelCommand, func(*exec.Cmd) (io.Reader, chan error, error) {
				errChan := make(chan error)
				close(errChan)
				contents, err := ioutil.ReadFile(tc.queryOutputFile)
				if err != nil {
					panic(fmt.Sprintf("failed to read query output test file %q: %v", tc.queryOutputFile, err))
				}
				return bytes.NewReader(contents), errChan, nil
			})
			defer stubs.Reset()

			w, err := OpenWorkspace("")
			if err != nil {
				t.Errorf("got error while opening workspace: %v; want no error", err)
				return
			}

			got, gotErr := w.Query("") // args don't matter

			errdiff.Check(t, gotErr, tc.wantErr)
			if gotErr != nil {
				return
			}

			assert.Equal(t, tc.wantCount, len(got.Targets))
		})
	}
}
