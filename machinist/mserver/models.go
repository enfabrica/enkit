package mserver

import (
	"net"
	"time"
)

type Node struct {
	Name string
	Tags []string
	Ips  []net.IP
}

type ReservedNode struct {
	*Node
	User  string
	End   time.Time
	Start time.Time
}
