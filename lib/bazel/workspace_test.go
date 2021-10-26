package bazel

import (
	"os/exec"
	"testing"

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
			stubs := gostub.Stub(&runBazelCommand, func(*exec.Cmd) (string, error) { return "", nil })
			defer stubs.Reset()

			w, err := OpenWorkspace("", tc.baseOpts...)
			if err != nil {
				t.Errorf("got error %v; want no error", err)
				return
			}

			q := &queryOptions{query: cannedQuery}
			tc.queryOpts.apply(q)
			got := w.bazelCommand(q)

			assert.Equal(t, tc.wantArgs, got.Args)
		})
	}
}
