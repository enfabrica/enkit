package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	apb "github.com/enfabrica/enkit/allocation_manager/proto"
)

// returns invocation used for the given topology name
func getInvocation(topology_name string, suffix string, checkin int64) *invocation {
	return &invocation{
		ID:          	 "id" + suffix,
		Owner:       	 "kjw" + suffix,
		Purpose:     	 "purpose" + suffix,
		LastCheckin: 	 time.Unix(checkin, 0),
		TopologyRequest: &apb.TopologyRequest{
			Name:   &topology_name,
		},
	}
}

func getInvA() *invocation {
	return getInvocation("topoA", "A", 1234567)
}

func getInvB() *invocation {
	return getInvocation("topoB", "B", 2345678)
}

func _unit_for(hostname string) *Unit {
	return &Unit{
		Health:   	apb.Health_HEALTH_READY,
		Invocation: nil,
		UnitInfo: 	apb.UnitInfo{
			Info: &apb.UnitInfo_HostInfo{
				HostInfo: &apb.HostInfo{Hostname: hostname},
			},
		},
	}
}

func getTestUnits() map[string]*Unit {
	return map[string]*Unit{
		"nameA": _unit_for("nameA"),
		"nameB": _unit_for("nameB"),
		"nameC": _unit_for("nameC"),
	}
}

func getTestInventory(units map[string]*Unit) *apb.HostInventory {
	inventory := &apb.HostInventory{Hosts: map[string]*apb.HostInfo{}}
	for hostname, _ := range units {
		inventory.GetHosts()[hostname] = &apb.HostInfo{Hostname: hostname}
	}
	return inventory
}

func getTestTopologies(units map[string]*Unit) map[string]*Topology {
	topologies := map[string]*Topology{
		"topoA": {
			Name: "topoA",
			Units: []*Unit{
				units["nameA"],
			},
		},
		"topoB": {
			Name: "topoB",
			Units: []*Unit{
				units["nameB"],
			},
		},
	}
	return topologies
}

func TestEnqueueOne(t *testing.T) {
	// setup
	iq := new(invocationQueue)
	invA := getInvA()
	// precondition
	assert.Equal(t, 0, iq.Len(), "iq before test")
	// test
	iq.Enqueue(invA)
	// verify
	assert.Equal(t, 1, iq.Len(), "iq")
	i, pos := iq.Get("idA")
	assert.Equal(t, Position(1), pos, "Position 1")
	assert.Equal(t, "idA", i.ID, "i.ID")
	assert.Equal(t, "kjwA", i.Owner, "i.Owner")
	assert.Equal(t, "purposeA", i.Purpose, "i.Purpose")
	assert.Equal(t, time.Unix(1234567, 0), i.LastCheckin, "i.LastCheckin")
	assert.Equal(t, "topoA", i.TopologyRequest.GetName(), "i.TopologyRequest.GetName()")
}

func TestEnqueueTwo(t *testing.T) {
	// setup
	iq := new(invocationQueue)
	invA := getInvA()
	invB := getInvB()
	// test
	iq.Enqueue(invA)
	assert.Equal(t, 1, iq.Len(), "iq middle of test")
	iq.Enqueue(invB)
	// verify
	assert.Equal(t, 2, iq.Len(), "iq")
	// don't test "ID:id", that was done by TestEnqueueOne.
	i, pos := iq.Get("idB")
	assert.NotNil(t, i, "idB not found")
	assert.Equal(t, Position(2), pos, "Position(2)")
	assert.Equal(t, "idB", i.ID, "i.ID")
	assert.Equal(t, "kjwB", i.Owner, "i.Owner")
	assert.Equal(t, "purposeB", i.Purpose, "i.Purpose")
	assert.Equal(t, time.Unix(2345678, 0), i.LastCheckin, "i.LastCheckin")
	assert.Equal(t, "topoB", i.TopologyRequest.GetName(), "i.TopologyRequest.GetName()")
}

func TestDequeue(t *testing.T) {
	// setup
	iq := new(invocationQueue)
	invA := getInvA()
	iq.Enqueue(invA)
	assert.Equal(t, 1, iq.Len(), "iq.Len()")
	i, _ := iq.Get("idA")
	assert.Equal(t, "kjwA", i.Owner, "i.Owner")
	// test
	inv := iq.Dequeue()
	// verify
	assert.Equal(t, "kjwA", inv.Owner, "inv.Owner")
	assert.Equal(t, 0, iq.Len(), "iq.Len()")
}

func TestExpireQueued(t *testing.T) {
	// setup
	iq := new(invocationQueue)
	invA := getInvA()
	invA.LastCheckin = time.Unix(1000, 0)
	iq.Enqueue(invA)
	assert.Equal(t, 1, iq.Len(), "iq.Len() before")
	// test
	iq.ExpireQueued(time.Unix(999, 0))
	assert.Equal(t, 1, iq.Len(), "iq.Len() after 999")
	iq.ExpireQueued(time.Unix(1000, -1))
	assert.Equal(t, 1, iq.Len(), "iq.Len() after 1000, -1")
	iq.ExpireQueued(time.Unix(1000, 0))
	assert.Equal(t, 0, iq.Len(), "iq.Len() after 1000, 0")
}

func TestSwap(t *testing.T) {
	// setup
	iq := new(invocationQueue)
	invA := getInvA()
	invB := getInvB()
	iq.Enqueue(invA)
	iq.Enqueue(invB)
	a, posA := iq.Get("idA")
	assert.Equal(t, "kjwA", a.Owner, "a.Owner before test")
	assert.Equal(t, Position(1), posA, "PositionA before test")
	b, posB := iq.Get("idB")
	assert.Equal(t, "kjwB", b.Owner, "b.Owner before test")
	assert.Equal(t, Position(2), posB, "PositionB before test")
	// test
	iq.Swap(0, 1)
	// verify
	b, posB = iq.Get("idB")
	assert.Equal(t, "kjwB", b.Owner, "b.Owner after swap")
	assert.Equal(t, Position(1), posB, "PositionB after swap")
	a, posA = iq.Get("idA")
	assert.Equal(t, "kjwA", a.Owner, "a.Owner after swap")
	assert.Equal(t, Position(2), posA, "PositionA after swap")
}

// TODO: upgrade this after Matchmaker() is finished
func TestPromote(t *testing.T) {
	// setup
	units := getTestUnits()
	inventory := getTestInventory(units)
	topologies := getTestTopologies(units)	

	iq := new(invocationQueue)
	invA := getInvA()
	invB := getInvB()
	iq.Enqueue(invA)
	iq.Enqueue(invB)
	// test
	iq.Promote(units, inventory, topologies)
	// verify
	assert.Equal(t, 0, iq.Len(), "iq.Len()")
	var invNil *invocation // assert.Equal tests type; this provides typing
	inv, pos := iq.Get("idA")
	assert.Equal(t, Position(0), pos, "id PositionA")
	assert.Equal(t, invNil, inv, "id inv")
	inv, pos = iq.Get("idB")
	assert.Equal(t, Position(0), pos, "idB PositionB")
	assert.Equal(t, invNil, inv, "idB inv")
}
