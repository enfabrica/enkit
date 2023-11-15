// +build windows
package kcerts

import (
	"net"
	"github.com/Microsoft/go-winio"
)
// When talking to the SSH agent on linux machines, use unix sockets
// while use named pipes for windows machines.
// https://learn.microsoft.com/en-us/windows/win32/ipc/named-pipes
func DialTimeout(a SSHAgent) (net.Conn, error) {
	return winio.DialPipe(a.State.Socket, &a.timeout)
}
