package testutil

import (
	"fmt"

	"github.com/bazelbuild/rules_go/go/tools/bazel"
)

func MustRunfile(path string) string {
	p, err := bazel.Runfile(path)
	if err != nil {
		panic(fmt.Sprintf("unable to find runfile %q: %v", path, err))
	}
	return p
}
