package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	apb "github.com/enfabrica/enkit/allocation_manager/proto"
)

func restoreTimeNow() { timeNow = time.Now }

func newService(currentState state) Service {
	units := getTestUnits()
	inventory := getTestInventory(units)
	topologies := getTestTopologies(units)

	return Service{
		currentState:              currentState,
		units:                     units,
		inventory:				   inventory,
		topologies:				   topologies,
		queueRefreshDuration:      100 * time.Second, // nanos
		allocationRefreshDuration: 200 * time.Second,
	}
}

func newRunningService() Service {
	return newService(stateRunning)
}

func newStartingService() Service {
	return newService(stateStarting)
}

// Test Allocate, Refresh, Release sequence
// TODO: rewrite after Matchmaker handles yaml
func TestServiceAllocate(t *testing.T) {
	defer restoreTimeNow()
	timeNow = func() time.Time { return time.Unix(10, 0) }
	s := newRunningService()
	ctx := context.Background()

	topo_name := "topoA"

	// Allocate
	inv := &apb.Invocation{Request: &apb.TopologyRequest{Name: &topo_name}}
	allocateResponse, err := s.Allocate(ctx, &apb.AllocateRequest{
		Invocation: inv,
	})
	assert.Equalf(t, nil, err, "Allocate returned error: %v", err)
	assert.NotEqualf(t, (*apb.AllocateResponse)(nil), allocateResponse, "allocateResponse: %v", allocateResponse)
	assert.Equal(t, (*apb.Queued)(nil), allocateResponse.GetQueued(), "allocateResponse.GetQueued")
	assert.NotEqualf(t, (*apb.Allocated)(nil), allocateResponse.GetAllocated(), "allocateResponse.GetAllocated: %v", allocateResponse.GetAllocated())
	assert.NotEqualf(t, "", allocateResponse.GetAllocated().GetId(), "allocateResponse.GetAllocated.GetId: %v", allocateResponse.GetAllocated().GetId())
	assert.Equal(t, int64(210), allocateResponse.GetAllocated().GetRefreshDeadline().GetSeconds(), "allocateResponse.GetAllocated.GetRefreshDeadline")
	assert.NotNil(t, s.units["nameA"], "s.units[\"nameA\"]")
	assert.NotNil(t, s.units["nameA"].Invocation, "s.units[\"nameA\"]: %v", s.units["nameA"].Invocation)
	// Capture ID before moving on to Refresh
	inv.Id = allocateResponse.GetAllocated().GetId()

	// Refresh
	timeNow = func() time.Time { return time.Unix(20, 0) }
	refreshResponse, err := s.Refresh(ctx, &apb.RefreshRequest{
		Invocation: inv,
		Allocated:  allocateResponse.GetAllocated().GetTopology(),
	})
	assert.Equalf(t, nil, err, "Refresh returned error: %v", err)
	assert.NotEqualf(t, (*apb.RefreshResponse)(nil), refreshResponse, "refreshResponse: %v", refreshResponse)
	assert.Equal(t, inv.Id, refreshResponse.GetId(), "refreshResponse")
	assert.Equal(t, int64(220), refreshResponse.GetRefreshDeadline().GetSeconds(), "refreshResponse.GetRefreshDeadline")

	// Release
	releaseResponse, err := s.Release(ctx, &apb.ReleaseRequest{
		Id: inv.Id,
	})
	assert.Equalf(t, nil, err, "Release returned error: %v", err)
	assert.NotEqualf(t, (*apb.ReleaseResponse)(nil), releaseResponse, "releaseResponse: %v", releaseResponse)
	// empty proto, nothing else to check
}

// Test Allocate, Allocate(Queued) sequence
// TODO: rewrite after Matchmaker handles yaml
func TestServiceAllocateQueued(t *testing.T) {
	defer restoreTimeNow()
	s := newRunningService()
	ctx := context.Background()

	topo_name := "topoA"
	topo_request := apb.TopologyRequest{Name: &topo_name}

	// Allocate
	firstInv := &apb.Invocation{Request: &topo_request}
	timeNow = func() time.Time { return time.Unix(10, 0) }
	allocateResponse, err := s.Allocate(ctx, &apb.AllocateRequest{
		Invocation: firstInv,
	})
	assert.Equalf(t, nil, err, "First Allocate returned error: %v", err)
	assert.NotEqualf(t, (*apb.AllocateResponse)(nil), allocateResponse, "first allocateResponse: %v", allocateResponse)
	assert.Equal(t, (*apb.Queued)(nil), allocateResponse.GetQueued(), "first allocateResponse.GetQueued")
	assert.NotEqualf(t, (*apb.Allocated)(nil), allocateResponse.GetAllocated(), "first allocateResponse.GetAllocated: %v", allocateResponse.GetAllocated())
	assert.NotEqualf(t, "", allocateResponse.GetAllocated().GetId(), "first allocateResponse.GetAllocated.GetId: %v", allocateResponse.GetAllocated().GetId())
	assert.Equal(t, int64(210), allocateResponse.GetAllocated().GetRefreshDeadline().GetSeconds(), "allocateResponse.GetAllocated.GetRefreshDeadline")
	assert.Equal(t, "topoA", allocateResponse.GetAllocated().GetTopology().GetName(), "allocateResponse.GetAllocated.GetTopology.GetName")

	// post condition of the first Allocate, precondition of the second Allocate
	assert.NotEqualf(t, (*invocation)(nil), s.units["nameA"].Invocation, "s.units[\"nameA\"]: %v", s.units["nameA"].Invocation)
	assert.Equal(t, true, s.units["nameA"].IsAllocated(), "s.units[\"nameA\"].IsAllocated")
	// Capture ID before moving on
	firstInv.Id = allocateResponse.GetAllocated().GetId()

	secondInv := &apb.Invocation{Request: &topo_request}
	// precondition, because I made a stupid mistake with pointers
	assert.Equal(t, "", secondInv.Id, "secondInv.Id")
	timeNow = func() time.Time { return time.Unix(11, 0) }
	allocateResponse, err = s.Allocate(ctx, &apb.AllocateRequest{
		Invocation: secondInv,
	})
	assert.Equalf(t, nil, err, "second Allocate returned error: %v", err)
	assert.NotEqualf(t, (*apb.AllocateResponse)(nil), allocateResponse, "second allocateResponse: %v", allocateResponse)
	assert.Equal(t, (*apb.Allocated)(nil), allocateResponse.GetAllocated(), "second allocateResponse.GetAllocated")
	assert.NotEqualf(t, (*invocation)(nil), s.units["nameA"].Invocation, "s.units[\"nameA\"].Invocation: %v", s.units["nameA"].Invocation)
	assert.Equal(t, true, s.units["nameA"].IsAllocated(), "s.units[\"nameA\"].IsAllocated")
	assert.NotEqualf(t, (*apb.Queued)(nil), allocateResponse.GetQueued(), "second allocateResponse.GetQueued: %v", allocateResponse.GetQueued())
	assert.Equal(t, int64(111), allocateResponse.GetQueued().GetNextPollTime().GetSeconds(), "second allocateResponse.GetQueued.GetNextPollTime")
	secondInv.Id = allocateResponse.GetQueued().GetId()
	assert.NotEqualf(t, secondInv.Id, firstInv.Id, "validate first != second ID")

	// Release
	timeNow = func() time.Time { return time.Unix(20, 0) }
	releaseResponse, err := s.Release(ctx, &apb.ReleaseRequest{
		Id: firstInv.Id,
	})
	assert.Equalf(t, nil, err, "Release returned error: %v", err)
	assert.NotEqualf(t, (*apb.ReleaseResponse)(nil), releaseResponse, "releaseResponse: %v", releaseResponse)
	assert.Equal(t, false, s.units["nameA"].IsAllocated(), "s.units[\"nameA\"].IsAllocated")
	// empty proto, nothing else to check

	// Trigger janitor run, to promote our Queued request
	s.janitor()

	// retry second Allocate
	timeNow = func() time.Time { return time.Unix(11, 0) }
	allocateResponse, err = s.Allocate(ctx, &apb.AllocateRequest{
		Invocation: secondInv,
	})
	assert.Equalf(t, nil, err, "repeat second Allocate returned error: %v", err)
	assert.NotEqualf(t, (*apb.AllocateResponse)(nil), allocateResponse, "repeat second allocateResponse: %v", allocateResponse)
	assert.Equal(t, (*apb.Queued)(nil), allocateResponse.GetQueued(), "repeat second allocateResponse.GetQueued")
	assert.NotEqualf(t, (*apb.Allocated)(nil), allocateResponse.GetAllocated(), "repeat second allocateResponse.GetAllocated: %v", allocateResponse.GetAllocated())
	assert.NotEqualf(t, "", allocateResponse.GetAllocated().GetId(), "repeat second allocateResponse.GetAllocated.GetId: %v", allocateResponse.GetAllocated().GetId())
	assert.Equal(t, int64(211), allocateResponse.GetAllocated().GetRefreshDeadline().GetSeconds(), "repeat second allocateResponse.GetAllocated.GetRefreshDeadline")
	assert.Equal(t, "topoA", allocateResponse.GetAllocated().GetTopology().GetName(), "allocateResponse.GetAllocated.GetTopology.GetName")
	// stop; skip testing Release
	// only test that Allocate(topoA) -> Queued -> Allocate(id)... works
}

// Test allocation request that can never be satisfied
func TestImpossibleAllocate(t *testing.T) {
	s := newRunningService()
	ctx := context.Background()

	topo_name := "nonesuch"

	// Allocate
	inv := &apb.Invocation{Request: &apb.TopologyRequest{Name: &topo_name}}
	allocateResponse, err := s.Allocate(ctx, &apb.AllocateRequest{
		Invocation: inv,
	})
	assert.NotNil(t, err, "Allocate returned error: %v", err)
	assert.Contains(t, err.Error(), "impossible to match against inventory", "impossible to match against inventory")
	assert.Nil(t, allocateResponse, "allocateResponse")	
}

// timeouts from queue
// timeouts from unit (eviction?)
// evictions ?!?

// Test Startup; Refresh
func TestServiceStartingRefresh(t *testing.T) {
	defer restoreTimeNow()
	timeNow = func() time.Time { return time.Unix(10, 0) }
	s := newStartingService()
	ctx := context.Background()

	topo_name := "topoA"

	// Refresh during startup -> Adopted (Allocated)
	inv := &apb.Invocation{Request: &apb.TopologyRequest{Name: &topo_name}, Id: "kjw"}
	refreshResponse, err := s.Refresh(ctx, &apb.RefreshRequest{
		Invocation: inv,
		Allocated:  &apb.Topology{Name: topo_name},
	})
	assert.Equalf(t, nil, err, "Refresh returned error: %v", err)
	assert.NotEqualf(t, (*apb.RefreshResponse)(nil), refreshResponse, "refreshResponse: %v", refreshResponse)
	assert.Equal(t, "kjw", refreshResponse.GetId(), "refreshResponse.GetID")
	assert.Equal(t, int64(210), refreshResponse.GetRefreshDeadline().GetSeconds(), "refreshResponse.GetRefreshDeadline")
	// only way to know what happened is to inspect s.units
	assert.NotEqualf(t, (*invocation)(nil), s.units["nameA"].Invocation, "s.units[\"nameA\"].Invocation: %v", s.units["nameA"].Invocation)
	assert.Equal(t, true, s.units["nameA"].IsAllocated(), "s.units[\"nameA\"].IsAllocated")
}

// Test Startup; Allocate
func TestServiceStartingFirstAllocate(t *testing.T) {
	defer restoreTimeNow()
	timeNow = func() time.Time { return time.Unix(10, 0) }
	s := newStartingService()
	ctx := context.Background()

	topo_name := "topoA"
	topo_request := apb.TopologyRequest{Name: &topo_name}

	// Allocate; first time should only Queue
	inv := &apb.Invocation{Request: &topo_request} // first time; no ID
	allocateResponse, err := s.Allocate(ctx, &apb.AllocateRequest{
		Invocation: inv,
	})
	assert.Equalf(t, nil, err, "Allocate returned error: %v", err)
	assert.NotEqualf(t, (*apb.AllocateResponse)(nil), allocateResponse, "allocateResponse: %v", allocateResponse)
	assert.NotEqualf(t, (*apb.Queued)(nil), allocateResponse.GetQueued(), "allocateResponse.GetQueued")
	assert.Equal(t, int64(110), allocateResponse.GetQueued().GetNextPollTime().GetSeconds(), "second allocateResponse.GetQueued.GetNextPollTime")
	assert.Equal(t, (*invocation)(nil), s.units["nameA"].Invocation, "s.units[\"nameA\"].Invocation")

	// Allocate; second attempt should also Queue
	timeNow = func() time.Time { return time.Unix(11, 0) }
	inv = &apb.Invocation{Request: &topo_request, Id: "kjw"} // second request; with id
	allocateResponse, err = s.Allocate(ctx, &apb.AllocateRequest{
		Invocation: inv,
	})
	assert.Equalf(t, nil, err, "Allocate returned error: %v", err)
	assert.NotEqualf(t, (*apb.AllocateResponse)(nil), allocateResponse, "allocateResponse: %v", allocateResponse)
	assert.NotEqualf(t, (*apb.Queued)(nil), allocateResponse.GetQueued(), "allocateResponse.GetQueued")
	assert.Equal(t, int64(111), allocateResponse.GetQueued().GetNextPollTime().GetSeconds(), "second allocateResponse.GetQueued.GetNextPollTime")
	assert.Equal(t, (*invocation)(nil), s.units["nameA"].Invocation, "s.units[\"nameA\"].Invocation")
}
