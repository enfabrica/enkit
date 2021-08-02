package kdns

import "net"

type IFlags interface {
	DnsFlags() *Flags
}

type Flags struct {
	Listener net.Listener
}
