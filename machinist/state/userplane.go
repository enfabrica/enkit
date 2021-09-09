package state

import (
	"net"
	"sync"
)

type UserPlaneMachine struct {
	Machine
	UserTags []string
	Reserved bool
}

type UserPlane struct {
	sync.Mutex `json:"-"`
	Machines []*UserPlaneMachine
}
// TODO(adam) refactor when generics come up
func mergeTags(in ...[]string)  []string{
	var all []string
	for _, t := range in {
		all = append(all , t...)
	}
	allKeys := make(map[string]bool)
	var list []string
	for _, item := range all {
		if _, value := allKeys[item]; !value {
			allKeys[item] = true
			list = append(list, item)
		}
	}
	return list
}

func mergeIps(in... []net.IP) []net.IP{
	var all []net.IP
	for _, t := range in {
		all = append(all , t...)
	}
	allKeys := make(map[string]bool)
	var list []net.IP
	for _, item := range all {
		if _, value := allKeys[item.String()]; !value {
			allKeys[item.String()] = true
			list = append(list, item)
		}
	}
	return list
}

// MergeStates will merge the list of machines into the userplane state. It will use the new machines <name,ip> pair as a source of truth.
// For example, if we have machines with {name: Foo, ip: Bar}, {name: Baz, ip: FooBar} and input {name: Foo, ip: FooBar}, {name: Baz, ip: Foo}
// it will result in {name: Foo, ip: Bar}, {name: Baz, ip: FooBar}
func MergeStates(up *UserPlane, machines []*Machine) *UserPlane {
	var mergedMachines []*UserPlaneMachine
	for _, savedMachine := range up.Machines {
		for _, m := range machines {
			if savedMachine.Name == m.Name {
				savedMachine.Ips = mergeIps(savedMachine.Ips, m.Ips)
				savedMachine.Tags = mergeTags(savedMachine.Tags, m.Tags)
			}
		}
		mergedMachines = append(mergedMachines, savedMachine)
	}
	return &UserPlane{
		Machines: mergedMachines,
	}
}


