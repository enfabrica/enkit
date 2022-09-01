package stamp

import (
	"strconv"
	"strings"
	"time"
)

var (
	BuildUser    = "<unknown>"
	GitBranch    = "<unknown>"
	GitSha       = "<unknown>"
	GitMasterSha = "<unknown>"

	changedFiles   = "<unknown>"
	buildTimestamp = "<unknown>"
)

func IsClean() bool {
	return strings.TrimSpace(changedFiles) == ""
}

func IsOfficial() bool {
	return strings.TrimSpace(changedFiles) == "" && GitBranch == "master"
}

func BuildTimestamp() time.Time {
	ts, err := strconv.ParseInt(buildTimestamp, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(ts, 0)
}
