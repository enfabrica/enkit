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
