package service

import (
	"context"
	"sync"
	"time"

	lmpb "github.com/enfabrica/enkit/license_manager/proto"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Service implements the LicenseManager gRPC service.
type Service struct {
	mu           sync.Mutex          // Protects the following members from concurrent access
	currentState state               // State of the server
	licenses     map[string]*license // Queues and allocations, managed per-license-type

	queueRefreshDuration      time.Duration // Queue entries not refreshed within this duration are expired
	allocationRefreshDuration time.Duration // Allocations not refreshed within this duration are expired
}

func New() *Service {
	service := &Service{
		currentState: stateStarting,
		// TODO: Read this from a config file and pass it in
		licenses: map[string]*license{
			"xilinx::foo": &license{
				totalAvailable: 2,
				allocations:    map[string]*invocation{},
			},
			"xilinx::bar": &license{
				totalAvailable: 2,
				allocations:    map[string]*invocation{},
			},
		},
		// TODO: Read this from flags
		queueRefreshDuration:      5 * time.Second,
		allocationRefreshDuration: 5 * time.Second,
	}

	go func(s *Service) {
		// TODO: Read this from flags
		t := time.NewTicker(1 * time.Second)
		defer t.Stop()
		for {
			<-t.C
			s.janitor()
		}
	}(service)

	go func(s *Service) {
		// TODO: Read this from flags
		<-time.After(10 * time.Second)
		s.mu.Lock()
		defer s.mu.Unlock()
		s.currentState = stateRunning
	}(service)

	return service
}

// invocation maps to a particular command invocation that has requested a
// license, and its associated metadata.
type invocation struct {
	ID          string    // Server-generated unique ID
	Owner       string    // Client-provided owner
	BuildTag    string    // Client-provided build tag. May not be unique across invocations
	LastCheckin time.Time // Time the invocation last had its queue position/allocation refreshed.
}

type state int

const (
	// Startup state during which server "adopts" unknown allocations. This is a
	// relatively short period (roughly 2x a refresh period) which helps
	// transition existing invocations in the event of a server restart without
	// unnecessarily cancelling invocations.
	stateStarting state = iota
	// Normal operating state.
	stateRunning
)

var (
	// generateRandomID returns a UUIDv4 string, and can be stubbed out for unit
	// tests.
	generateRandomID = func() (string, error) {
		id, err := uuid.NewRandom()
		if err != nil {
			return "", err
		}
		return id.String(), nil
	}

	// timeNow returns the current time, and can be stubbed out for unit tests.
	timeNow = time.Now
)

// janitor runs in a loop to cleanup allocations and queue spots that have not
// been refreshed in a sufficient amount of time, as well as to promote queued
// licenses to allocations.
func (s *Service) janitor() {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Don't expire or promote anything during startup.
	if s.currentState == stateStarting {
		return
	}
	allocationExpiry := timeNow().Add(-s.allocationRefreshDuration)
	queueExpiry := timeNow().Add(-s.queueRefreshDuration)
	for _, lic := range s.licenses {
		lic.ExpireAllocations(allocationExpiry)
		lic.ExpireQueued(queueExpiry)
		lic.Promote()
	}
}

// Allocate allocates a license to the requesting invocation, or queues the
// request if none are available. See the proto docstrings for more details.
func (s *Service) Allocate(ctx context.Context, req *lmpb.AllocateRequest) (*lmpb.AllocateResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(req.GetLicenses()) != 1 {
		return nil, status.Errorf(codes.InvalidArgument, "licenses must have exactly one license spec")
	}
	licenseType := formatLicenseType(req.GetLicenses()[0])
	lic, ok := s.licenses[licenseType]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "unknown license type: %q", licenseType)
	}
	invocationID := req.GetInvocationId()

	if invocationID == "" {
		// This is the first AllocationRequest by this invocation. Generate an ID
		// and queue it.
		var err error
		invocationID, err = generateRandomID()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to generate invocation_id: %v", err)
		}
		inv := &invocation{
			ID:          invocationID,
			Owner:       req.GetOwner(),
			BuildTag:    req.GetBuildTag(),
			LastCheckin: timeNow(),
		}
		lic.Enqueue(inv)

		if s.currentState == stateRunning {
			lic.Promote()
		}
	}

	// Invocation ID should now be known and either queued (due to the above
	// insert, or from a previous request) or allocated (promoted from the queue
	// by above, or asynchronously by the janitor).

	if inv := lic.GetAllocated(invocationID); inv != nil {
		// Invocation is allocated
		inv.LastCheckin = timeNow()
		return &lmpb.AllocateResponse{
			ResponseType: &lmpb.AllocateResponse_LicenseAllocated{
				LicenseAllocated: &lmpb.LicenseAllocated{
					InvocationId:           invocationID,
					LicenseRefreshDeadline: timestamppb.New(timeNow().Add(s.allocationRefreshDuration)),
				},
			},
		}, nil
	}
	if inv := lic.GetQueued(invocationID); inv != nil {
		// Invocation is queued
		inv.LastCheckin = timeNow()
		return &lmpb.AllocateResponse{
			ResponseType: &lmpb.AllocateResponse_Queued{
				Queued: &lmpb.Queued{
					InvocationId: invocationID,
					NextPollTime: timestamppb.New(timeNow().Add(s.queueRefreshDuration)),
				},
			},
		}, nil
	}
	// Invocation is not allocated or queued
	if s.currentState == stateRunning {
		// This invocation is unknown (possibly expired)
		return nil, status.Errorf(codes.FailedPrecondition, "invocation_id not found: %q", invocationID)
	}
	// This invocation was previously queued before the server restart; add it
	// back to the queue.
	inv := &invocation{
		ID:          invocationID,
		Owner:       req.GetOwner(),
		BuildTag:    req.GetBuildTag(),
		LastCheckin: timeNow(),
	}
	lic.Enqueue(inv)
	return &lmpb.AllocateResponse{
		ResponseType: &lmpb.AllocateResponse_Queued{
			Queued: &lmpb.Queued{
				InvocationId: invocationID,
				NextPollTime: timestamppb.New(timeNow().Add(s.queueRefreshDuration)),
			},
		},
	}, nil
}

// Refresh serves as a keepalive to refresh an allocation while an invocation
// is still using it. See the proto docstrings for more info.
func (s *Service) Refresh(ctx context.Context, req *lmpb.RefreshRequest) (*lmpb.RefreshResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(req.GetLicenses()) != 1 {
		return nil, status.Errorf(codes.InvalidArgument, "licenses must have exactly one license spec")
	}
	licenseType := formatLicenseType(req.GetLicenses()[0])
	lic, ok := s.licenses[licenseType]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "unknown license type: %q", licenseType)
	}
	invID := req.GetInvocationId()
	if invID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invocation_id must be set")
	}
	inv := lic.GetAllocated(invID)
	if inv == nil {
		if s.currentState == stateRunning {
			return nil, status.Errorf(codes.FailedPrecondition, "invocation_id not allocated: %q", invID)
		}
		// "Adopt" this invocation and allocate it a license, if possible.
		inv := &invocation{
			ID:          invID,
			Owner:       req.GetOwner(),
			BuildTag:    req.GetBuildTag(),
			LastCheckin: timeNow(),
		}
		if ok := lic.Allocate(inv); ok {
			return &lmpb.RefreshResponse{
				InvocationId:           invID,
				LicenseRefreshDeadline: timestamppb.New(timeNow().Add(s.allocationRefreshDuration)),
			}, nil
		} else {
			return nil, status.Errorf(codes.ResourceExhausted, "%q has no available licenses", licenseType)
		}
	}
	// Update the time and return the next check interval
	inv.LastCheckin = timeNow()
	return &lmpb.RefreshResponse{
		InvocationId:           invID,
		LicenseRefreshDeadline: timestamppb.New(timeNow().Add(s.allocationRefreshDuration)),
	}, nil
}

// Release returns an allocated license and/or unqueues the specified
// invocation ID across all license types. See the proto docstrings for more
// details.
func (s *Service) Release(ctx context.Context, req *lmpb.ReleaseRequest) (*lmpb.ReleaseResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	invID := req.GetInvocationId()
	if invID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invocation_id must be set")
	}
	count := 0
	for _, lic := range s.licenses {
		count += lic.Forget(invID)
	}
	if count == 0 {
		return nil, status.Errorf(codes.FailedPrecondition, "invocation_id not found: %q", invID)
	}
	return &lmpb.ReleaseResponse{}, nil
}

// LicensesStatus returns the status for every license type. See the proto
// docstrings for more details.
func (s *Service) LicensesStatus(ctx context.Context, req *lmpb.LicensesStatusRequest) (*lmpb.LicensesStatusResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	res := &lmpb.LicensesStatusResponse{}
	for name, lic := range s.licenses {
		res.LicenseStats = append(res.LicenseStats, lic.GetStats(name))
	}
	return res, nil
}
