// +build !release

package ktest

import (
	"fmt"
	"github.com/pkg/errors"
	"net"
)

func AllocatePort() (*net.TCPAddr, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, err
	}
	allocatedPort, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return nil, errors.New("shape of the address not correct, is your os not unix?")
	}
	return allocatedPort, nil
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

	*k = append(*k, process...)
}


func (k *KillAbleProcess) Add(p func()){
	*k = append(*k, p)
}
