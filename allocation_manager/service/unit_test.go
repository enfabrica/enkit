package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	apb "github.com/enfabrica/enkit/allocation_manager/proto"
)

func unitA() unit {
	return unit{
		Health: apb.Health_HEALTH_READY,
		Topology: apb.Topology{
			Name: "Unit Name", Config: "Unit Config",
		},
		Invocation: nil,
	}
}

func invX() ([]*apb.Topology, *invocation) {
	topos := []*apb.Topology{}
	topos = append(topos, &apb.Topology{
		Name: "nameX", Config: "yamlX",
	})
	return topos, &invocation{
		ID:          "idX",
		Owner:       "ownerX",
		Purpose:     "purposeX",
		LastCheckin: time.Unix(1000, 0),
		QueueID:     1,
		Topologies:  topos,
	}
}

func TestAllocate(t *testing.T) {
	// setup
	u := unitA()
	topos, inv := invX()
	var iNil *invocation
	// precondition
	assert.Equal(t, iNil, u.Invocation, "u.Invocation before")
	// test
	assert.Equal(t, true, u.Allocate(inv), "u.Allocate(inv)")
	// verify
	assert.NotEqual(t, iNil, u.Invocation, "u.Invocation after")
	assert.Equal(t, apb.Health_HEALTH_READY, u.Health, "u.Invocation.Topologies")
	assert.Equal(t, "Unit Name", u.Topology.GetName(), "u.Topology.GetName()")
	assert.Equal(t, "Unit Config", u.Topology.GetConfig(), "u.Topology.GetConfig()")
	inv = u.Invocation
	if inv != nil {
		assert.Equal(t, "idX", inv.ID, "u.Invocation.ID")
		assert.Equal(t, "ownerX", inv.Owner, "u.Invocation.Owner")
		assert.Equal(t, "purposeX", inv.Purpose, "u.Invocation.Purpose")
		assert.Equal(t, time.Unix(1000, 0), inv.LastCheckin, "u.Invocation.LastCheckin")
		assert.Equal(t, QueueID(1), inv.QueueID, "u.Invocation.QueueID")
		assert.Equal(t, topos, inv.Topologies, "u.Invocation.Topologies")
	}
	// test re-add
	assert.Equal(t, false, u.Allocate(inv), "u.Allocate(inv) second insert")
}

func TestIsAllocated(t *testing.T) {
	// setup
	u := unitA()
	_, inv := invX()
	var iNil *invocation
	// precondition
	assert.Equal(t, iNil, u.Invocation, "u.Invocation before")
	assert.Equal(t, false, u.IsAllocated(), "u.IsAllocated()")
	// test
	assert.Equal(t, true, u.Allocate(inv), "u.Allocate(inv)")
	// verify
	assert.NotEqual(t, iNil, u.Invocation, "u.Invocation after")
	assert.Equal(t, true, u.IsAllocated(), "u.IsAllocated()")
	// test re-add
	assert.Equal(t, false, u.Allocate(inv), "u.Allocate(inv) second insert")
	assert.Equal(t, true, u.IsAllocated(), "u.IsAllocated()")
}

func TestIsHealthy(t *testing.T) {
	// setup
	u := unitA()
	// test
	u.Health = apb.Health_HEALTH_UNINITIALIZED
	assert.Equal(t, false, u.IsHealthy(), "u(Health_HEALTH_UNINITIALIZED)")
	u.Health = apb.Health_HEALTH_UNKNOWN
	assert.Equal(t, true, u.IsHealthy(), "u(Health_HEALTH_UNKNOWN)")
	u.Health = apb.Health_HEALTH_READY
	assert.Equal(t, true, u.IsHealthy(), "u(Health_HEALTH_READY)")
	u.Health = apb.Health_HEALTH_SIDELINED
	assert.Equal(t, false, u.IsHealthy(), "u(Health_HEALTH_SIDELINED)")
	u.Health = apb.Health_HEALTH_BROKEN
	assert.Equal(t, true, u.IsHealthy(), "u(Health_HEALTH_BROKEN)")
}

func TestGetInvocation(t *testing.T) {
	// setup
	u := unitA()
	_, inv := invX()
	var iNil *invocation
	// precondition
	assert.Equal(t, iNil, u.Invocation, "u.Invocation before")
	assert.Equal(t, iNil, u.GetInvocation("idX"), "u.GetInvocation(\"idX\")")
	// test
	assert.Equal(t, true, u.Allocate(inv), "u.Allocate(inv)")
	// verify
	assert.NotEqual(t, iNil, u.Invocation, "u.Invocation after")
	assert.NotEqual(t, iNil, u.GetInvocation("idX"), "u.GetInvocation(idX)")
	assert.Equal(t, iNil, u.GetInvocation("nonesuch"), "u.GetInvocation(nonesuch)")
}

func TestExpireAllocations(t *testing.T) {
	// setup
	u := unitA()
	_, inv := invX()
	var iNil *invocation
	u.Allocate(inv)
	// precondition
	assert.NotEqual(t, iNil, u.GetInvocation("idX"), "u.GetInvocation(\"idX\") before ExpireAllocations(999)")
	// test
	u.ExpireAllocations(time.Unix(999, 0))
	assert.NotEqual(t, iNil, u.GetInvocation("idX"), "u.GetInvocation(\"idX\") after ExpireAllocations(999)")
	u.ExpireAllocations(time.Unix(1000, -1))
	assert.NotEqual(t, iNil, u.GetInvocation("idX"), "u.GetInvocation(\"idX\") after ExpireAllocations(1000, -1)")
	u.ExpireAllocations(time.Unix(1000, 0))
	assert.Equal(t, iNil, u.GetInvocation("idX"), "u.GetInvocation(\"idX\") after ExpireAllocations(1000)")
}

func TestForget(t *testing.T) {
	// setup
	u := unitA()
	_, inv := invX()
	var iNil *invocation
	u.Allocate(inv)
	// precondition
	assert.NotEqual(t, iNil, u.GetInvocation("idX"), "before u.Forget")
	// test
	assert.Equal(t, 1, u.Forget("idX"), "u.Forget")
	assert.Equal(t, iNil, u.GetInvocation("idX"), "after u.Forget")
	assert.Equal(t, 0, u.Forget("idX"), "u.Forget second try")
}

func TestNew(t *testing.T) {
	u := unitA()
	assert.Equal(t, u.Topology.Name, "Unit Name")

	topo := apb.Topology{
		Name: "Unit Name 2", Config: "Unit Config",
	}
	a := newUnit(topo)
	assert.Equal(t, a.Topology.Name, "Unit Name 2")
	a.DoOperation("allocate")
	a.DoOperation("release")
	u.DoOperation("allocate")
	a.DoOperation("allocate")
	a.DoOperation("release")

	// fmt.Printf("metrics:", a.Metrics)
	// assert.True(t, false)
}

// TODO: test u.GetStats
