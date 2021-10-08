package bazel

import (
	"testing"

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
			w, err := OpenWorkspace("", tc.baseOpts...)
			if err != nil {
				t.Errorf("got error %v; want no error")
				return
			}

			q := &queryOptions{query: cannedQuery}
			tc.queryOpts.apply(q)
			got := w.bazelCommand(q)

			assert.Equal(t, tc.wantArgs, got.Args)
		})
	}
}
