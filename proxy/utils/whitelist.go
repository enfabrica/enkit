package utils

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/oauth"
	"github.com/enfabrica/enkit/proxy/nasshp"
	"path/filepath"
)

type PatternList []string

// NewPatternList creates an object capable of checking if the destination protocol, host, and port are allowed.
//
// allowed is a list of strings, each string is a pattern valid as per filepath.Match.
//
// When a connection is attempted by an user, a key made of "proto|host:ip" is checked against each element
// of the whitelist. If any of those element matches, the connection is allowed. If not, it is rejected.
//
// Some example patterns:
// - "*:22" -> allow any connection to port 22.
// - "tcp|*:22" -> allow any tcp connection to port 22.
// - "tcp|10.10.0.12:*" -> allow connecting to 10.10.0.12 on any port.
// - "tcp|10.10.0.12:22" -> allow connecting to 10.10.0.12 on port 22.
// - "tcp|10.10.*.*:22" -> allow connecting to any host in 10.10.0.0/16 on port 22.
//
func NewPatternList(allowed []string) (PatternList, error) {
	// Iterate over the list just to ensure that the patterns are valid.
	for _, pattern := range allowed {
		_, err := filepath.Match(pattern, "")
		if err != nil {
			return nil, fmt.Errorf("invalid pattern %s in list: %w", pattern, err)
		}
	}
	return PatternList(allowed), nil
}

// Allow is a nasshp.Filter function that returns nasshp.VerdictAllow if the proto and hostport
// string specified match any pattern in the list created with NewPatternList.
func (pl PatternList) Allow(proto string, hostport string, creds *oauth.CredentialsCookie) nasshp.Verdict {
	key := proto + "|" + hostport
	for _, pattern := range pl {
		match, err := filepath.Match(pattern, key)
		if err == nil && match {
			return nasshp.VerdictAllow
		}
	}
	return nasshp.VerdictDrop
}
