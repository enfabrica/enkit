// +build !release

package ktest

import (
	"errors"
	"fmt"
	"net"
)

type PortDescriptor struct {
	net.Listener
}

func (d PortDescriptor) Addr() (*net.TCPAddr, error) {
	allocatedDatastorePort, ok := d.Listener.Addr().(*net.TCPAddr)
	if !ok {
		return nil, errors.New("shape of the address not correct, is your os not unix?")
	}
	return allocatedDatastorePort, nil
}

func AllocatePort() (*PortDescriptor, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, err
	}
	return &PortDescriptor{listener}, nil
}

type KillAbleProcess []func()

func (k *KillAbleProcess) KillAll() {
	fmt.Println("running kill functions")
	for _, killFunc := range *k {
		fmt.Println("running kill function")
		killFunc()
	}
}

func (k *KillAbleProcess) AddKillable(process KillAbleProcess) {
	if k == nil {
		return
	}
	newList := append(*k, process...)
	*k = newList
}

func (k *KillAbleProcess) Add(p func()) {
	newList := append(*k, p)
	*k = newList
}
