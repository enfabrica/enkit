package errdiff

import (
	"fmt"
	"strings"
	"testing"
)

func substring(got error, want string) string {
	switch {
	case got == nil && want == "":
		return ""
	case got == nil && want != "":
		return fmt.Sprintf("got no error; want error containing %q", want)
	case got != nil && want == "":
		return fmt.Sprintf("got error: '%v'; want no error", got)
	case got != nil && want != "":
		if strings.Contains(got.Error(), want) {
			return ""
		}
		return fmt.Sprintf("got error: '%v'; want error containing substring: %q", got, want)
	}
	panic("unhandled case")
}

// Check checks that an error `got` matches the expectation `want`.
// Empty `want` string means no error is expected; if not empty, `got` must be
// non-nil and contain `want` as a substring to meet the expectation.
// Returns an empty string if `got` meets the `want` expectation, or a string
// containing an explanation of the discrepancy otherwise.
func Check(t *testing.T, got error, want string) {
	t.Helper()
	if diff := substring(got, want); diff != "" {
		t.Error(diff)
	}
}
