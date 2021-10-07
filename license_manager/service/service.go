package service

import (
	"context"
	"strings"
	"sync"
	"time"

	lmpb "github.com/enfabrica/enkit/license_manager/proto"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Service struct {
	mu           sync.Mutex
	currentState state
	licenses     map[string]*license

	queueRefreshDuration      time.Duration
	allocationRefreshDuration time.Duration
}

type license struct {
	totalAvailable int
	queue          []*invocation
	allocations    map[string]*invocation
}

type invocation struct {
	ID          string
	Owner       string
	BuildTag    string
	LastCheckin time.Time
}

type state int

const (
	stateStarting state = iota
	stateRunning
)

var (
	generateRandomID = func() (string, error) {
		id, err := uuid.NewRandom()
		if err != nil {
			return "", err
		}
		return id.String(), nil
	}

	timeNow = time.Now
)

// janitor runs in a loop to cleanup allocations and queue spots that have not
// been refreshed in a sufficient amount of time.
func (s *Service) janitor() {
	// Pick a time t = now + n to expire queue spots and allocation requests
	// For each license type
	//   For each allocation
	//     If allocation is not refreshed by deadline, remove it
	//     Place it in the "recently expired" list
	// For each license type
	//   For each queued invocation, first to last
	//     If queue spot is not refreshed by deadline, remove it
	//       Place it in the "recently expired" list
	//     If state == RUNNING and allocation is available, pop off queue and move it to allocated
	// For each recently expired allocation
	//   If past threshold, delete
}

func formatLicenseType(l *lmpb.License) string {
	return strings.Join([]string{l.GetVendor(), l.GetFeature()}, "::")
}

func (l *license) Enqueue(inv *invocation) {
	l.queue = append(l.queue, inv)
}

func (l *license) Promote() {
	numFree := l.totalAvailable - len(l.allocations)
	numAllocated := 0
	for i := 0; i < numFree && i < len(l.queue); i++ {
		l.allocations[l.queue[i].ID] = l.queue[i]
		numAllocated++
	}
	l.queue = l.queue[numAllocated:]
}

func (l *license) GetAllocated(invID string) *invocation {
	inv, ok := l.allocations[invID]
	if !ok {
		return nil
	}
	return inv
}

func (l *license) GetQueued(invID string) *invocation {
	for _, inv := range l.queue {
		if inv.ID == invID {
			return inv
		}
	}
	return nil
}

func (l *license) RefreshAllocated(invID string) bool {
	inv, ok := l.allocations[invID]
	if !ok {
		return false
	}
	inv.LastCheckin = timeNow()
	return true
}

func (s *Service) Allocate(ctx context.Context, req *lmpb.AllocateRequest) (*lmpb.AllocateResponse, error) {
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

		switch s.currentState {
		case stateStarting:
		case stateRunning:
			lic.Promote()
		default:
			return nil, status.Errorf(codes.Internal, "unhandled state: %v", s.currentState)
		}
	}

	// Is the invocation_id already allocated?
	if inv := lic.GetAllocated(invocationID); inv != nil {
		inv.LastCheckin = timeNow()
		//     Yes - return allocation success
		return &lmpb.AllocateResponse{
			ResponseType: &lmpb.AllocateResponse_LicenseAllocated{
				LicenseAllocated: &lmpb.LicenseAllocated{
					InvocationId:           invocationID,
					LicenseRefreshDeadline: timestamppb.New(timeNow().Add(s.allocationRefreshDuration)),
				},
			},
		}, nil
	}
	// Is the invocation_id already queued?
	if inv := lic.GetQueued(invocationID); inv != nil {
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
	switch s.currentState {
	case stateStarting:
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
	case stateRunning:
		return nil, status.Errorf(codes.FailedPrecondition, "invocation_id not found: %q", invocationID)
	default:
		return nil, status.Errorf(codes.Internal, "state not handled: %v", s.currentState)
	}
}

func (s *Service) refreshAll(invID string) int {
	refreshCount := 0
	for _, lic := range s.licenses {
		if lic.RefreshAllocated(invID) {
			refreshCount++
		}
	}
	return refreshCount
}

func (s *Service) Refresh(ctx context.Context, req *lmpb.RefreshRequest) (*lmpb.RefreshResponse, error) {
	invID := req.GetInvocationId()
	if invID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invocation_id must be set")
	}
	numUpdated := s.refreshAll(invID)
	if numUpdated == 0 {
		return nil, status.Errorf(codes.FailedPrecondition, "invocation_id not allocated a license: %v", invID)
	}
	// If state == STARTUP
	//   Get invocation_id
	//     No invocation_id -> error
	//   Is the invocation_id allocated?
	//     Yes - return allocation success
	//     Update last check time
	//   invocation_id is not allocated, but assumed to be allocated. Allocate if there is room
	//     Error if no allocations left
	//
	// If state == RUNNING
	//   Get invocation_id
	//     No invocation_id -> error
	//   Is the invocation_id allocated?
	//     Yes - return refresh response
	//     Update last check time
	//   Is the invocation_id recently expired?
	//     Yes - log metric
	//   invocation_id is not allocated - return error
	return nil, status.Errorf(codes.Unimplemented, "Refresh() is not yet implemented")
}

func (s *Service) Release(ctx context.Context, req *lmpb.ReleaseRequest) (*lmpb.ReleaseResponse, error) {
	// Get invocation_id
	//   No invocation_id -> error
	// Is the invocation_id allocated?
	//   Yes - remove allocation
	// invocation_id is not allocated - return error
	return nil, status.Errorf(codes.Unimplemented, "Release() is not yet implemented")
}

func (s *Service) LicensesStatus(ctx context.Context, req *lmpb.LicensesStatusRequest) (*lmpb.LicensesStatusResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "LicensesStatus() is not yet implemented")
}
