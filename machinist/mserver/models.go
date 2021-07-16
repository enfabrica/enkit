package mserver

import "net"

type Node struct {
	Name string
	Tags []string
	Ips  []net.IP
}
