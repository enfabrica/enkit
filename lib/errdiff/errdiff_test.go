package errdiff

import (
	"errors"
	"testing"
)

func TestSubstring(t *testing.T) {
	testCases := []struct {
		desc string
		err error
		subStr string
		wantDiff bool
	} {
		{
			desc: "pass with nil error and no substring expectation",
			err: nil,
			subStr: "",
			wantDiff: false,
		},
		{
			desc: "pass when non-nil error contains substring",
			err: errors.New("some error text"),
			subStr: "error text",
			wantDiff: false,
		},
		{
			desc: "fail when non-nil error doesn't contain substring",
			err: errors.New("some error text"),
			subStr: "some other error",
			wantDiff: true,
		},
		{
			desc: "fail when non-nil error and empty substring expectation",
			err: errors.New("some error text"),
			subStr: "",
			wantDiff: true,
		},
		{
			desc: "fail when nil error and non-empty substring expectation",
			err: nil,
			subStr: "error text",
			wantDiff: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func (t *testing.T) {
			diff := Substring(tc.err, tc.subStr)
			if tc.wantDiff && diff == "" {
				t.Error("got no diff; wanted diff")
			}
			if !tc.wantDiff && diff != "" {
				t.Errorf("got diff %q; want no diff", diff)
			}
		})
	}
}