package kdns

import "net"

type IFlags interface {
	DnsFlags() *Flags
}

type Flags struct {
	TCPListener net.Listener
	UDPListener net.PacketConn
}
