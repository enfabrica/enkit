package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	apb "github.com/enfabrica/enkit/allocation_manager/proto"
)

func invX() *invocation {
	topo_name := "nameX"

	return &invocation{
		ID:          		"idX",
		Owner:       		"ownerX",
		Purpose:     		"purposeX",
		LastCheckin: 		time.Unix(1000, 0),
		QueueID:     		1,
		TopologyRequest:  	&apb.TopologyRequest{Name: &topo_name},
	}
}

func TestAllocate(t *testing.T) {
	// setup
	u := getTestUnits()["nameA"]

	inv := invX()
	var iNil *invocation
	// precondition
	assert.Equal(t, iNil, u.Invocation, "u.Invocation before")
	// test
	assert.Equal(t, true, u.Allocate(inv), "u.Allocate(inv)")
	// verify
	assert.NotEqual(t, iNil, u.Invocation, "u.Invocation after")
	assert.Equal(t, apb.Health_HEALTH_READY, u.Health, "u.Invocation.Topologies")
	assert.Equal(t, "nameA", u.GetName(), "u.GetName")
	switch info := u.UnitInfo.Info.(type) {
		case *apb.UnitInfo_HostInfo:
			assert.Equal(t, "nameA", info.HostInfo.GetHostname(), "info.HostInfo.GetName")
	}	
	inv = u.Invocation
	if inv != nil {
		assert.Equal(t, "idX", inv.ID, "u.Invocation.ID")
		assert.Equal(t, "ownerX", inv.Owner, "u.Invocation.Owner")
		assert.Equal(t, "purposeX", inv.Purpose, "u.Invocation.Purpose")
		assert.Equal(t, time.Unix(1000, 0), inv.LastCheckin, "u.Invocation.LastCheckin")
		assert.Equal(t, QueueID(1), inv.QueueID, "u.Invocation.QueueID")
	}
	// test re-add
	assert.Equal(t, false, u.Allocate(inv), "u.Allocate(inv) second insert")
}

func TestIsAllocated(t *testing.T) {
	// setup
	u := getTestUnits()["nameA"]
	inv := invX()
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
	u := getTestUnits()["nameA"]
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
	u := getTestUnits()["nameA"]
	inv := invX()
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
	u := getTestUnits()["nameA"]
	inv := invX()
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
	u := getTestUnits()["nameA"]
	inv := invX()
	var iNil *invocation
	u.Allocate(inv)
	// precondition
	assert.NotEqual(t, iNil, u.GetInvocation("idX"), "before u.Forget")
	// test
	assert.Equal(t, 1, u.Forget("idX"), "u.Forget")
	assert.Equal(t, iNil, u.GetInvocation("idX"), "after u.Forget")
	assert.Equal(t, 0, u.Forget("idX"), "u.Forget second try")
}

// TODO: test u.GetStats
