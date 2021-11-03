package bazel

import (
	"io"
	"os/exec"
	"strings"
	"testing"

	"github.com/enfabrica/enkit/lib/errdiff"

	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"
)

func TestBazelQueryCommand(t *testing.T) {
	cannedQuery := "deps(//...)"
	testCases := []struct {
		desc      string
		baseOpts  BaseOptions
		queryOpts QueryOptions
		wantArgs  []string
		wantErr   string
	}{
		{
			desc:      "basic query",
			baseOpts:  nil,
			queryOpts: nil,
			wantArgs:  []string{"bazel", "query", "--output=streamed_proto", "--", cannedQuery},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			var gotCmd *exec.Cmd
			stubs := gostub.Stub(&NewCommand, func(cmd *exec.Cmd) (Command, error) {
				gotCmd = cmd
				return &fakeCommand{
					stdout: io.NopCloser(strings.NewReader("")),
					stderr: nil,
				}, nil
			})
			defer stubs.Reset()

			w, err := OpenWorkspace("", tc.baseOpts...)
			if err != nil {
				t.Errorf("got error %v; want no error", err)
				return
			}

			q := &queryOptions{query: cannedQuery}
			tc.queryOpts.apply(q)
			_, gotErr := w.bazelCommand(q)

			errdiff.Check(t, gotErr, tc.wantErr)
			assert.Equal(t, tc.wantArgs, gotCmd.Args)
		})
	}
}
