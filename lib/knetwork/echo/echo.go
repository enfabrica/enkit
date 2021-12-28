// Package echo implements a naive echo server.
//
// The echo server opens a port of choice, it spawns a goroutine for every
// incoming connection, and echos back any data written into it.
//
// This is a textbook, naive, implementation of an echo server: no timeouts,
// no limiting of incoming connections, no facilities to terminate the
// clients as well as the listening socket on stop.
//
// This is most useful for testing code that requires a TCP endpoint on
// the other end.
package echo

import (
	"fmt"
	"io"
	"net"
)

type Echo struct {
	listener net.Listener
}

func New(address string) (*Echo, error) {
	l, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}

	return &Echo{listener: l}, nil
}

func (e *Echo) Close() error {
	return e.listener.Close()
}

func (e *Echo) Address() (*net.TCPAddr, error) {
	if e.listener == nil {
		return nil, fmt.Errorf("invalid listener: must be initialized with New()")
	}

	addr := e.listener.Addr()

	taddr, ok := addr.(*net.TCPAddr)
	if !ok {
		return nil, fmt.Errorf("internal error: could not convert address to TCPAddr")
	}

	return taddr, nil
}

func (e *Echo) Run() error {
	for {
		conn, err := e.listener.Accept()
		if err != nil {
			return err
		}

		go e.handleConnection(conn)
	}
}

func (e *Echo) handleConnection(conn net.Conn) {
	defer conn.Close()
	io.Copy(conn, conn)
}
