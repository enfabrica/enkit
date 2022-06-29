package stamp

import (
	"strings"
)

var (
	BuildUser    = "<unknown>"
	GitBranch    = "<unknown>"
	GitSha       = "<unknown>"
	GitMasterSha = "<unknown>"

	changedFiles = "<unknown>"
)

func IsClean() bool {
	return strings.TrimSpace(changedFiles) == ""
}

func IsOfficial() bool {
	return strings.TrimSpace(changedFiles) == "" && GitBranch == "master"
}
