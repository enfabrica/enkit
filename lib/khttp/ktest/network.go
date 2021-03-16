// +build !release

package ktest

import (
	"fmt"
)

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
