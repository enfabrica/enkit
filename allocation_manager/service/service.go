package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	apb "github.com/enfabrica/enkit/allocation_manager/proto"
	"github.com/enfabrica/enkit/lib/logger"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	metricRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Subsystem: "allocation_manager",
		Name:      "request_duration_seconds",
		Help:      "RPC execution time as seen by the server",
	},
		[]string{
			"method",
			"response_code",
		},
	)
	metricJanitorDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Subsystem: "allocation_manager",
		Name:      "janitor_duration_seconds",
		Help:      "Janitor execution time",
	})
	metricRequestCodes = promauto.NewCounterVec(prometheus.CounterOpts{
		Subsystem: "allocation_manager",
		Name:      "response_count",
		Help:      "Total number of response codes sent",
	},
		[]string{
			"method",
			"response_code",
		},
	)
	// service state
	// qty allocated
	// queue length
)

// Service implements the LicenseManager gRPC service.
type Service struct {
	mu                        sync.Mutex       // Protects the following members from concurrent access
	currentState              state            // State of the server
	units                     map[string]*unit // Managed Units
	queueRefreshDuration      time.Duration    // Queue entries not refreshed within this duration are expired
	allocationRefreshDuration time.Duration    // Allocations not refreshed within this duration are expired
}

// UnitsFromConfig given a Config proto during startup, store unit topologies
func UnitsFromConfig(config *apb.Config) (map[string]*unit, error) {
	failure := 0
	names := make(map[string]bool)
	units := make(map[string]*unit)
	for _, uMsg := range config.GetUnits() {
		// config := uMsg.GetTopology().GetConfig()
		// TODO: parse yaml, pull unique name out of config
		// parsed :=...
		// name := parsed.name
		// if name != parsed.name {
		//  return units, fmt.Errorf("Error: topology name %s does not match config name %s", uMsg.GetName(), parsed.name)}
		// until then, assume name is correct
		name := uMsg.GetTopology().GetName()
		/* TODO queue
		var prioritizer Prioritizer
		switch uMsg.Prioritizer.(type) {
		case *apb.UnitConfig_Fifo:
			prioritizer = &FIFOPrioritizer{}
		case *apb.UnitConfig_EvenOwners:
			prioritizer = NewEvenOwnersPrioritizer()
		default:
			prioritizer = &FIFOPrioritizer{}
		}
		*/
		u := new(unit)
		u.Health = apb.Health_HEALTH_UNKNOWN // janitor must mark machine as clean
		u.Topology = *uMsg.Topology
		// u.prioritizer = prioritizer
		_, ok := names[name]
		if ok {
			failure = failure + 1
			logger.Go.Errorf("Duplicate Unit name: %s", name)
		} else {
			names[name] = true
			// Must turn units off before renaming them, or startup Adoption will fail
			// TODO: document this procedure
			units[name] = u
		}
	}
	if failure > 0 {
		return nil, fmt.Errorf("Error: %d Unit topology names not unique, expected 0", failure)
	}
	return units, nil
}

func defaultUint32(v, d uint32) uint32 {
	if v == 0 {
		return d
	}
	return v
}

func New(config *apb.Config) (*Service, error) {
	if config.GetServer() == nil {
		return nil, fmt.Errorf("missing `server` section in config")
	}
	queueRefreshSeconds := defaultUint32(config.GetServer().GetQueueRefreshDurationSeconds(), 15)
	allocationRefreshSeconds := defaultUint32(config.GetServer().GetAllocationRefreshDurationSeconds(), 30)
	janitorIntervalSeconds := defaultUint32(config.GetServer().GetJanitorIntervalSeconds(), 1)
	adoptionDurationSeconds := defaultUint32(config.GetServer().GetAdoptionDurationSeconds(), 45)
	units, err := UnitsFromConfig(config)
	if err != nil {
		return nil, err
	}

	service := &Service{
		currentState:              stateStarting,
		units:                     units,
		queueRefreshDuration:      time.Duration(queueRefreshSeconds) * time.Second,
		allocationRefreshDuration: time.Duration(allocationRefreshSeconds) * time.Second,
	}

	go func(s *Service) {
		t := time.NewTicker(time.Duration(janitorIntervalSeconds) * time.Second)
		defer t.Stop()
		for {
			<-t.C
			s.janitor()
		}
	}(service)

	go func(s *Service) {
		<-time.After(time.Duration(adoptionDurationSeconds) * time.Second)
		s.mu.Lock()
		defer s.mu.Unlock()
		s.currentState = stateRunning
	}(service)

	return service, nil
}

// QueueID is a monotonically increasing number representing the absolute
// position of the item in the queue from when the queue was last emptied.
//
// If the element is reordered, so to be dequeued earlier, its QueueID
// will be changed accordingly.
type QueueID uint64

// Position represents a relative position within the queue.
//
// For example, if this is the 3rd element in the queue, Position will be 3.
//
// Given the QueueID of an element its Position can be computed in O(1) by
// subtracting the QueueID of the first element currently in the queue.
type Position uint32

// invocation maps to a particular command invocation that has requested a
// license, and its associated metadata.
type Reservation struct {
	InvocationID string
	UnitID       string
	Invocation   apb.Invocation
	Start        time.Time
	Stop         time.Time
	Owner        string    // Client-provided build tag. May not be unique across invocations
	purpose      string    // Client-provided build tag. May not be unique across invocations
	LastCheckin  time.Time // Time the invocation last had its queue position/allocation refreshed.
	// DeferReleaseSeconds time.Duration
}

type invocation struct {
	ID          string    // Server-generated unique ID
	Owner       string    // Client-provided owner
	Purpose     string    // Client-provided purpose (CI: send test target)
	LastCheckin time.Time // Time the invocation last had its queue position/allocation refreshed.
	QueueID     QueueID   // Position in the queue. 0 means the invocation has not been queued yet.
	Topologies  []*apb.Topology
}

func (i *invocation) ToProto() *apb.Invocation {
	return &apb.Invocation{
		Owner: i.Owner,
		// Purpose: i.purpose,
		Id: i.ID,
	}
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
	defer updateJanitorMetrics(timeNow())
	s.mu.Lock()
	defer s.mu.Unlock()
	// Don't expire or promote anything during startup.
	if s.currentState == stateStarting {
		return
	}
	now := timeNow()
	allocationExpiry := now.Add(-s.allocationRefreshDuration)
	// queueExpiry := now.Add(-s.queueRefreshDuration)
	for _, u := range s.units {
		u.ExpireAllocations(allocationExpiry)
		// u.ExpireQueued(queueExpiry)  // TODO queue
		// u.Promote()  // TODO queue
	}
}

func updateJanitorMetrics(startTime time.Time) {
	d := timeNow().Sub(startTime)
	metricJanitorDuration.Observe(d.Seconds())
}

func updateMetrics(method string, err *error, startTime time.Time) {
	d := timeNow().Sub(startTime)
	code := status.Code(*err)
	metricRequestCodes.WithLabelValues(method, code.String()).Inc()
	metricRequestDuration.WithLabelValues(method, code.String()).Observe(d.Seconds())
}

func Matchmaker(units map[string]*unit, req *apb.Invocation, all bool) ([]*unit, error) {
	matches := []*unit{}
	reqTopos := req.GetTopologies()
	if len(reqTopos) != 1 {
		return nil, fmt.Errorf("only 1 topology request allowed (for now), got %d", reqTopos)
	}
	nMatches := 0
	for _, t := range reqTopos { // yes, premature bundle code, but doesn't seem to hurt
		newMatches := []*unit{}
		for _, u := range units {
			// if !all && u.Invocation != nil { append... }
			// TODO add priority...
			if u.Topology.GetName() == t.GetName() { // PROTOTYPE ONLY
				newMatches = append(newMatches, u)
				if all {
					break
				}
			}
		}
		if len(newMatches) > 0 {
			nMatches += 1
			matches = append(matches, newMatches...)
		}
	}
	if nMatches < len(reqTopos) {
		return matches, fmt.Errorf("matched %d of %d requests", len(matches), len(reqTopos))
	}
	return matches, nil
}

// Allocate allocates a unit to the requesting invocation, or queues the
// request if none are available. See the proto docstrings for more details.
func (s *Service) Allocate(ctx context.Context, req *apb.AllocateRequest) (retRes *apb.AllocateResponse, retErr error) {
	defer updateMetrics("Allocate", &retErr, timeNow())
	s.mu.Lock()
	defer s.mu.Unlock()
	invMsg := req.GetInvocation()
	if len(invMsg.GetTopologies()) != 1 {
		return nil, status.Errorf(codes.InvalidArgument, "requests must have exactly one topology (for now)")
	}
	// TODO(kjw): Match requested topology against all configured topologies
	matches, err := Matchmaker(s.units, invMsg, true)
	if err != nil {
		return nil, err
	}
	// if NO possible match, status.Errorf(codes.?, "impossible to match topologies: %v", topos)
	//name := ExtractUnitName(invMsg.GetTopologies())
	//u, ok := s.units[name]
	//if !ok {
	//	return nil, status.Errorf(codes.NotFound, "unknown license type: %q", licenseType)
	//}
	matches, err = Matchmaker(s.units, invMsg, false)
	if err != nil {
		return nil, err
	}
	// assume we only get 1 back
	u := matches[0] // picked unit
	invocationID := invMsg.GetId()
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
			Owner:       invMsg.GetOwner(),
			Purpose:     invMsg.GetPurpose(),
			LastCheckin: timeNow(),
		}
		u.Allocate(inv)
		// TODO queue
		// u.Enqueue(inv)
		if s.currentState == stateRunning {
			// TODO queue
			// u.Promote()
		}
	}
	// Invocation ID should now be known and either queued (due to the above
	// insert, or from a previous request) or allocated (promoted from the queue
	// by above, or asynchronously by the janitor).
	topos := make([]*apb.Topology, 0)
	// for u := range(???)
	if inv := u.GetInvocation(invocationID); inv != nil {
		// Invocation is allocated
		inv.LastCheckin = timeNow()
		topos = append(topos, &u.Topology)
		return &apb.AllocateResponse{
			ResponseType: &apb.AllocateResponse_Allocated{
				Allocated: &apb.Allocated{
					Id:              invocationID,
					Topologies:      topos,
					RefreshDeadline: timestamppb.New(timeNow().Add(s.allocationRefreshDuration)),
				},
			},
		}, nil
	}
	// TODO move queue code
	/*
		if inv, pos := u.GetQueued(invocationID); inv != nil {
			// Invocation is queued
			inv.LastCheckin = timeNow()
			return &apb.AllocateResponse{
				ResponseType: &apb.AllocateResponse_Queued{
					Queued: &apb.Queued{
						Id:            invocationID,
						NextPollTime:  timestamppb.New(timeNow().Add(s.queueRefreshDuration)),
						QueuePosition: uint32(pos),
					},
				},
			}, nil
		}
	*/
	// Invocation is not allocated or queued
	if s.currentState == stateRunning {
		// This invocation is unknown (possibly expired)
		return nil, status.Errorf(codes.FailedPrecondition, "invocation_id not found: %q", invocationID)
	}
	// This invocation was previously queued before the server restart; add it
	// back to the queue.
	/*
		inv = &invoc{
			ID:    invocationID,
			Owner: inv.GetOwner(),
			//TODO: Purpose:    inv.GetPurpose(),
			LastCheckin: timeNow(),
		}
		// TODO move queue code
		//pos := u.Enqueue(inv)
	*/
	return &apb.AllocateResponse{
		ResponseType: &apb.AllocateResponse_Queued{
			Queued: &apb.Queued{
				Id:           invocationID,
				NextPollTime: timestamppb.New(timeNow().Add(s.queueRefreshDuration)),
				// QueuePosition: uint32(pos),
			},
		},
	}, nil
}

// Refresh serves as a keepalive to refresh an allocation while an invocation
// is still using it. See the proto docstrings for more info.
func (s *Service) Refresh(ctx context.Context, req *apb.RefreshRequest) (retRes *apb.RefreshResponse, retErr error) {
	defer updateMetrics("Refresh", &retErr, timeNow())
	s.mu.Lock()
	defer s.mu.Unlock()
	reqInvoc := req.GetInvocation()
	want := reqInvoc.GetTopologies() // repeated Topology, despite singular in name
	if len(want) != 1 {
		return nil, status.Errorf(codes.InvalidArgument, "request must have exactly one topology, got %d", len(want))
	}
	got := req.GetAllocated() // repeated Topology
	if len(got) != 1 {
		return nil, status.Errorf(codes.InvalidArgument, "allocations must have exactly one topology, got %d: %v", len(got), got)
	}
	// TODO: handle multiple topologies (bundles)
	// i.e. replace above with a len(want) == len(got) ?
	// for _, allocated := range(got) {
	allocated := got[0]
	name := allocated.Name

	u, ok := s.units[name]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "unknown unit name: %s", name)
	}
	// TODO: validate configs are the same, and name isn't just faked
	// perhaps: if u.config == topo.config {
	// but byte-to-byte identical yaml test can lead to adoption failure when new configs are deployed
	invID := reqInvoc.GetId()
	if invID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invocation.id must be set")
	}
	unitInvoc := u.GetInvocation(invID)
	if unitInvoc == nil {
		if s.currentState == stateRunning {
			return nil, status.Errorf(codes.FailedPrecondition, "invocation_id not allocated: %q", invID)
		}
		// else "Adopt" this invocation
		inv := &invocation{
			ID:         invID,
			Owner:      reqInvoc.GetOwner(),
			Purpose:    reqInvoc.GetPurpose(),
			Topologies: reqInvoc.GetTopologies(),
			// LastCheckin: timeNow(), redundant
		}
		if ok := u.Allocate(inv); !ok {
			return nil, status.Errorf(codes.ResourceExhausted, "%s cannot be allocated (adopted)", name)
		}
	}
	u.Invocation.LastCheckin = timeNow()
	return &apb.RefreshResponse{
		Id:              invID,
		RefreshDeadline: timestamppb.New(timeNow().Add(s.allocationRefreshDuration)),
	}, nil
}

// Release returns an allocated license and/or unqueues the specified
// invocation ID across all license types. See the proto docstrings for more
// details.
func (s *Service) Release(ctx context.Context, req *apb.ReleaseRequest) (retRes *apb.ReleaseResponse, retErr error) {
	defer updateMetrics("Release", &retErr, timeNow())
	s.mu.Lock()
	defer s.mu.Unlock()
	invID := req.GetId()
	if invID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invocation_id must be set")
	}
	count := 0
	for _, unit := range s.units {
		count += unit.Forget(invID)
	}
	if count == 0 {
		return nil, status.Errorf(codes.FailedPrecondition, "invocation_id not found: %q", invID)
	}
	return &apb.ReleaseResponse{}, nil
}

// Status returns the status for every license type. See the proto
// docstrings for more details.
func (s *Service) Status(ctx context.Context, req *apb.StatusRequest) (retRes *apb.StatusResponse, retErr error) {
	defer updateMetrics("Status", &retErr, timeNow())
	s.mu.Lock()
	defer s.mu.Unlock()
	res := &apb.StatusResponse{}
	for _, unit := range s.units {
		res.Stats = append(res.Stats, unit.GetStats())
	}

	//	// Sort by vendor, then feature, with two groups: first group has either
	//	// allocations or queued invocations, second group has neither.
	//	sort.Slice(res.Stats, func(i, j int) bool {
	//		if aHasEntries != bHasEntries {
	//			return aHasEntries
	//		}
	//		licA, licB := res.Stats[i].Get(), res.Stats[j].GetLicense()
	//		if licA.GetVendor() == licB.GetVendor() {
	//			return licA.GetFeature() < licB.GetFeature()
	//		}
	//		return licA.GetVendor() < licB.GetVendor()
	//	})

	return res, nil
}
