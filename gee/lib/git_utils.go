package lib

import (
	"strings"
)

func GetCurrentBranch() string {
	result := Runner().RunGit("rev-parse", "--abbrev-ref", "HEAD")
	if err := result.CheckExitCode(); err != nil {
		Logger().Fatalf("Error: %q", err)
	}
	return strings.TrimSpace(result.stdout.String())
}
