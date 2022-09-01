package version

import (
	"sort"
)

// Add entries to this list for every feature that we may want to be able to
// manually/programmatically detect.
//
// Features can be a dash-delimited string, like: `tunnel-local-uds`
// Features can be a bug fix, like: `INFRA-1234-fix`
var features = []string{
	"tunnel-local-uds", // Tunnels support listening on a UNIX domain socket locally
}

func init() {
	sort.Strings(features)
}

func featuresContains(entry string) bool {
	i := sort.SearchStrings(features, entry)
	return i < len(features) && features[i] == entry
}
