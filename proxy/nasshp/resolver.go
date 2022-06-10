package nasshp

import (
	"fmt"
	"net"
	"strconv"
)

// Resolver can resolve a host+port pair into a potentially different host+port
// pair.
type Resolver interface {
	Resolve(host, port string) (string, string, error)
}

// MultiResolver is a list of Resolvers that are applied in order until the list
// is exhausted or an error is encountered.
type MultiResolver []Resolver

func (r MultiResolver) Resolve(host, port string) (string, string, error) {
	var err error
	for _, resolver := range r {
		host, port, err = resolver.Resolve(host, port)
		if err != nil {
			return "", "", err
		}
	}
	return host, port, nil
}

// FailEmptyHost is a Resolver that errors if host is empty, but otherwise
// returns the host and port unmodified.
type FailEmptyHost struct{}

func (r *FailEmptyHost) Resolve(host, port string) (string, string, error) {
	if host == "" {
		return "", "", fmt.Errorf("invalid empty host %q", host)
	}
	return host, port, nil
}

// FailEmptyPort is a Resolver that errors if port is empty, but otherwise
// returns the host and port unmodified.
type FailEmptyPort struct{}

func (r *FailEmptyPort) Resolve(host, port string) (string, string, error) {
	if port == "" {
		return "", "", fmt.Errorf("invalid port %q", port)
	}
	return host, port, nil
}

// SRVResolver is a Resolver that attempts to resolve the port via an SRV record
// if the port is empty.
type SRVResolver struct{}

func (r *SRVResolver) Resolve(host, port string) (string, string, error) {
	if port != "" {
		return host, port, nil
	}
	_, srvs, err := net.LookupSRV("", "", host)
	if err != nil {
		return "", "", fmt.Errorf("SRV lookup for %q failed: %v", host, err)
	}
	if len(srvs) < 1 {
		return "", "", fmt.Errorf("no SRV records for host %q", host)
	}
	return host, strconv.Itoa(int(srvs[0].Port)), nil
}
