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

func formatLicenseType(l *lmpb.License) string {
	return strings.Join([]string{l.GetVendor(), l.GetFeature()}, "::")
}

func (l *license) Enqueue(inv *invocation) {
	l.queue = append(l.queue, inv)
}

func (l *license) Allocate(inv *invocation) bool {
	if len(l.allocations) >= l.totalAvailable {
		return false
	}
	l.allocations[inv.ID] = inv
	return true
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

func (l *license) ExpireAllocations(expiry time.Time) {
	newAllocations := map[string]*invocation{}
	for k, v := range l.allocations {
		if v.LastCheckin.After(expiry) {
			newAllocations[k] = v
		}
	}
	l.allocations = newAllocations
}

func (l *license) ExpireQueued(expiry time.Time) {
	newQueued := []*invocation{}
	for _, inv := range l.queue {
		if inv.LastCheckin.After(expiry) {
			newQueued = append(newQueued, inv)
		}
	}
	l.queue = newQueued
}

func (l *license) GetQueued(invID string) *invocation {
	for _, inv := range l.queue {
		if inv.ID == invID {
			return inv
		}
	}
	return nil
}

func (l *license) GetStats(name string) *lmpb.LicenseStats {
	fields := strings.SplitN(name, "::", 2)
	if len(fields) != 2 {
		fields = []string{"<UNKNOWN>", name}
	}
	return &lmpb.LicenseStats{
		License: &lmpb.License{
			Vendor:  fields[0],
			Feature: fields[1],
		},
		Timestamp:         timestamppb.New(timeNow()),
		TotalLicenseCount: uint32(l.totalAvailable),
		AllocatedCount:    uint32(len(l.allocations)),
		QueuedCount:       uint32(len(l.queue)),
	}
}

func (l *license) Forget(invID string) int {
	count := 0
	newAllocations := map[string]*invocation{}
	for k, v := range l.allocations {
		if k != invID {
			newAllocations[k] = v
		} else {
			count++
		}
	}
	l.allocations = newAllocations
	return count
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

func (s *Service) Refresh(ctx context.Context, req *lmpb.RefreshRequest) (*lmpb.RefreshResponse, error) {
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
	inv, ok := lic.allocations[invID]
	if !ok {
		switch s.currentState {
		case stateStarting:
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
		case stateRunning:
			return nil, status.Errorf(codes.FailedPrecondition, "invocation_id not allocated: %q", invID)
		}
	}
	inv.LastCheckin = timeNow()
	return &lmpb.RefreshResponse{
		InvocationId:           invID,
		LicenseRefreshDeadline: timestamppb.New(timeNow().Add(s.allocationRefreshDuration)),
	}, nil
}

func (s *Service) Release(ctx context.Context, req *lmpb.ReleaseRequest) (*lmpb.ReleaseResponse, error) {
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

func (s *Service) LicensesStatus(ctx context.Context, req *lmpb.LicensesStatusRequest) (*lmpb.LicensesStatusResponse, error) {
	res := &lmpb.LicensesStatusResponse{}
	for name, lic := range s.licenses {
		res.LicenseStats = append(res.LicenseStats, lic.GetStats(name))
	}
	return res, nil
}
