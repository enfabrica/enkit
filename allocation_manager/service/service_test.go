package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apb "github.com/enfabrica/enkit/allocation_manager/proto"
)

func restoreTimeNow() { timeNow = time.Now }

func newService(currentState state) Service {
	units := map[string]*unit{
		"unitA": {Topology: apb.Topology{Name: "unitA"}},
		"unitB": {Topology: apb.Topology{Name: "unitB"}},
	}
	return Service{
		currentState:              currentState,
		units:                     units,
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
	topos := []*apb.Topology{{Name: "unitA"}}

	// Allocate
	inv := &apb.Invocation{Topologies: topos}
	allocateResponse, err := s.Allocate(ctx, &apb.AllocateRequest{
		Invocation: inv,
	})
	assert.Equalf(t, nil, err, "Allocate returned error: %v", err)
	assert.NotEqualf(t, (*apb.AllocateResponse)(nil), allocateResponse, "allocateResponse: %v", allocateResponse)
	assert.Equal(t, (*apb.Queued)(nil), allocateResponse.GetQueued(), "allocateResponse.GetQueued")
	assert.NotEqualf(t, (*apb.Allocated)(nil), allocateResponse.GetAllocated(), "allocateResponse.GetAllocated: %v", allocateResponse.GetAllocated())
	assert.NotEqualf(t, "", allocateResponse.GetAllocated().GetId(), "allocateResponse.GetAllocated.GetId: %v", allocateResponse.GetAllocated().GetId())
	assert.Equal(t, int64(210), allocateResponse.GetAllocated().GetRefreshDeadline().GetSeconds(), "allocateResponse.GetAllocated.GetRefreshDeadline")
	assert.NotEqualf(t, (*invocation)(nil), s.units["unitA"].Invocation, "s.units[\"unitA\"]: %v", s.units["unitA"].Invocation)
	// Capture ID before moving on to Refresh
	inv.Id = allocateResponse.GetAllocated().GetId()

	// Refresh
	timeNow = func() time.Time { return time.Unix(20, 0) }
	refreshResponse, err := s.Refresh(ctx, &apb.RefreshRequest{
		Invocation: inv,
		Allocated:  allocateResponse.GetAllocated().GetTopologies(),
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
	topos := []*apb.Topology{{Name: "unitA"}}

	// Allocate
	firstInv := &apb.Invocation{Topologies: topos}
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
	assert.Equal(t, "unitA", allocateResponse.GetAllocated().GetTopologies()[0].GetName(), "allocateResponse.GetAllocated.GetTopologies[0].GetName")
	// post condition of the first Allocate, precondition of the second Allocate
	assert.NotEqualf(t, (*invocation)(nil), s.units["unitA"].Invocation, "s.units[\"unitA\"]: %v", s.units["unitA"].Invocation)
	assert.Equal(t, true, s.units["unitA"].IsAllocated(), "s.units[\"unitA\"].IsAllocated")
	// Capture ID before moving on
	firstInv.Id = allocateResponse.GetAllocated().GetId()

	secondInv := &apb.Invocation{Topologies: topos}
	// precondition, because I made a stupid mistake with pointers
	assert.Equal(t, "", secondInv.Id, "secondInv.Id")
	timeNow = func() time.Time { return time.Unix(11, 0) }
	allocateResponse, err = s.Allocate(ctx, &apb.AllocateRequest{
		Invocation: secondInv,
	})
	assert.Equalf(t, nil, err, "second Allocate returned error: %v", err)
	assert.NotEqualf(t, (*apb.AllocateResponse)(nil), allocateResponse, "second allocateResponse: %v", allocateResponse)
	assert.Equal(t, (*apb.Allocated)(nil), allocateResponse.GetAllocated(), "second allocateResponse.GetAllocated")
	assert.NotEqualf(t, (*invocation)(nil), s.units["unitA"].Invocation, "s.units[\"unitA\"].Invocation: %v", s.units["unitA"].Invocation)
	assert.Equal(t, true, s.units["unitA"].IsAllocated(), "s.units[\"unitA\"].IsAllocated")
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
	assert.Equal(t, false, s.units["unitA"].IsAllocated(), "s.units[\"unitA\"].IsAllocated")
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
	assert.Equal(t, "unitA", allocateResponse.GetAllocated().GetTopologies()[0].GetName(), "allocateResponse.GetAllocated.GetTopologies[0].GetName")
	// stop; skip testing Release
	// only test that Allocate(unitA) -> Queued -> Allocate(id)... works
}

// Test allocation request that can never be satisfied
func TestImpossibleAllocate(t *testing.T) {
	s := newRunningService()
	ctx := context.Background()
	topos := []*apb.Topology{{Name: "nonesuch"}}

	// Allocate
	inv := &apb.Invocation{Topologies: topos}
	allocateResponse, err := s.Allocate(ctx, &apb.AllocateRequest{
		Invocation: inv,
	})
	assert.NotEqualf(t, nil, err, "Allocate returned error: %v", err)
	// TODO: how to make this less brittle?
	assert.Equal(t, status.Errorf(codes.InvalidArgument, "results: 1 topologies + 2 units = [0] matches .  impossible to match against inventory. This is a permanent failure, not an availability failure."), err, "Allocate returned error")
	assert.Equal(t, (*apb.AllocateResponse)(nil), allocateResponse, "allocateResponse")
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
	topos := []*apb.Topology{{Name: "unitA"}}

	// Refresh during startup -> Adopted (Allocated)
	inv := &apb.Invocation{Topologies: topos, Id: "kjw"}
	refreshResponse, err := s.Refresh(ctx, &apb.RefreshRequest{
		Invocation: inv,
		Allocated:  []*apb.Topology{{Name: "unitA"}},
	})
	assert.Equalf(t, nil, err, "Refresh returned error: %v", err)
	assert.NotEqualf(t, (*apb.RefreshResponse)(nil), refreshResponse, "refreshResponse: %v", refreshResponse)
	assert.Equal(t, "kjw", refreshResponse.GetId(), "refreshResponse.GetID")
	assert.Equal(t, int64(210), refreshResponse.GetRefreshDeadline().GetSeconds(), "refreshResponse.GetRefreshDeadline")
	// only way to know what happened is to inspect s.units
	assert.NotEqualf(t, (*invocation)(nil), s.units["unitA"].Invocation, "s.units[\"unitA\"].Invocation: %v", s.units["unitA"].Invocation)
	assert.Equal(t, true, s.units["unitA"].IsAllocated(), "s.units[\"unitA\"].IsAllocated")
}

// Test Startup; Allocate
func TestServiceStartingFirstAllocate(t *testing.T) {
	defer restoreTimeNow()
	timeNow = func() time.Time { return time.Unix(10, 0) }
	s := newStartingService()
	ctx := context.Background()
	topos := []*apb.Topology{{Name: "unitA"}}

	// Allocate; first time should only Queue
	inv := &apb.Invocation{Topologies: topos} // first time; no ID
	allocateResponse, err := s.Allocate(ctx, &apb.AllocateRequest{
		Invocation: inv,
	})
	assert.Equalf(t, nil, err, "Allocate returned error: %v", err)
	assert.NotEqualf(t, (*apb.AllocateResponse)(nil), allocateResponse, "allocateResponse: %v", allocateResponse)
	assert.NotEqualf(t, (*apb.Queued)(nil), allocateResponse.GetQueued(), "allocateResponse.GetQueued")
	assert.Equal(t, int64(110), allocateResponse.GetQueued().GetNextPollTime().GetSeconds(), "second allocateResponse.GetQueued.GetNextPollTime")
	assert.Equal(t, (*invocation)(nil), s.units["unitA"].Invocation, "s.units[\"unitA\"].Invocation")

	// Allocate; second attempt should also Queue
	timeNow = func() time.Time { return time.Unix(11, 0) }
	inv = &apb.Invocation{Topologies: topos, Id: "kjw"} // second request; with id
	allocateResponse, err = s.Allocate(ctx, &apb.AllocateRequest{
		Invocation: inv,
	})
	assert.Equalf(t, nil, err, "Allocate returned error: %v", err)
	assert.NotEqualf(t, (*apb.AllocateResponse)(nil), allocateResponse, "allocateResponse: %v", allocateResponse)
	assert.NotEqualf(t, (*apb.Queued)(nil), allocateResponse.GetQueued(), "allocateResponse.GetQueued")
	assert.Equal(t, int64(111), allocateResponse.GetQueued().GetNextPollTime().GetSeconds(), "second allocateResponse.GetQueued.GetNextPollTime")
	assert.Equal(t, (*invocation)(nil), s.units["unitA"].Invocation, "s.units[\"unitA\"].Invocation")
}
