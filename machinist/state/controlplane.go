package state

import "sync"

type Machine struct {
}

type MachineController struct {
	sync.Mutex
	Machines []*Machine
}
