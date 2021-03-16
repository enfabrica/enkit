package knetwork

import (
	"fmt"
	"net"
)

type PortDescriptor struct {
	net.Listener
}

func (pd *PortDescriptor) Address() (*net.TCPAddr, error) {
	allocatedPort, ok := pd.Addr().(*net.TCPAddr)
	if !ok {
		return nil, fmt.Errorf("shape of address %v not correct, is not a net.TCPAddr", pd.Addr())
	}
	return allocatedPort, nil
}

func AllocatePort() (*PortDescriptor, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, err
	}
	return &PortDescriptor{listener}, nil
}
