package testutil

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
)

func AssertProtoEqual(t *testing.T, got interface{}, want interface{}) {
	t.Helper()
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("Proto messages do not match:\n%s\n", diff)
	}
}

// AssertCmp fails a test with a descriptive diff if got != want, respecting a
// set of compare options.
func AssertCmp(t *testing.T, got interface{}, want interface{}, opts ...cmp.Option) {
	t.Helper()
	if diff := cmp.Diff(want, got, opts...); diff != "" {
		t.Errorf("Objects are not equal:\n%s\n", diff)
	}
}
