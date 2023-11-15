// +build linux
package kcerts

import (
	"net"
)
func DialTimeout(a SSHAgent) (net.Conn, error) {
	return net.DialTimeout("unix", a.State.Socket, a.timeout)
}
