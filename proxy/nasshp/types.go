package nasshp

import (
	"net"
	"strings"
)

type HostIp string

func (hi HostIp) IsUrl() bool {
	res := strings.Split(string(hi), ":")
	if len(res) != 2 {
		return false
	}
	return net.ParseIP(res[0]) == nil
}

func (hi HostIp) Resolve() []string {
	res := strings.Split(string(hi), ":")
	if len(res) != 2 {
		return []string{}
	}
	p := res[1]
	ips, err := net.LookupHost(res[0])
	if err != nil {
		return []string{}
	}
	var toReturn []string
	for _, ip := range ips {
		toReturn = append(toReturn, strings.Join([]string{ip, ":", p}, ""))
	}
	return toReturn
}

type Verdict int

const (
	// Can't decide either way. This is useful for chaning filters.
	VerdictUnknown Verdict = iota
	// Let the request in.
	VerdictAllow
	// Block the request.
	VerdictDrop
)

func (v Verdict) MergePreferAllow(vv Verdict) Verdict {
	if v == VerdictAllow || vv == VerdictAllow {
		return VerdictAllow
	}
	return VerdictDrop
}
