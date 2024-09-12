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
	units                     map[string]*unit // Managed Units key=topology name (string) value=*unit
	queueRefreshDuration      time.Duration    // Queue entries not refreshed within this duration are expired
	allocationRefreshDuration time.Duration    // Allocations not refreshed within this duration are expired
}

// UnitsFromConfig given a Config proto during startup, store unit topologies
func UnitsFromConfig(config *apb.Config) (map[string]*unit, error) {
	failure := 0
	names := map[string]bool{}
	units := map[string]*unit{}
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

/*
// TODO: add these bits into invocation struct below
	Start        time.Time
	Stop         time.Time
	// DeferReleaseSeconds time.Duration
*/
// invocation contains the original request and metadata/terms of the Allocation
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
	InvocationQueue.Promote(s.units)
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

// TODO: move to topology.go
type Match struct {
	Topology *apb.Topology // yes, duplicate from invocation, but pointers are cheap. this makes iterating easier.
	Units    []*unit
	// bitmap bitmap.Bitmap
}

func (m *Match) Found() bool {
	return len(m.Units) > 0
}

type Matched struct {
	matches    []*Match // fixed size array; same size as invocation.Topologies
	invocation *invocation
}

// TODO: feels not quite right style wise
func NewMatched(inv *invocation) *Matched {
	return &Matched{
		invocation: inv,
		matches:    make([]*Match, len(inv.Topologies)), // n.b. fixed size array
	}
}

func (md *Matched) Found() bool {
	count := 0
	for _, match := range md.matches {
		// TODO: use bitmaps
		if match.Found() {
			count += 1
		}
	}
	return len(md.matches) == count
}

func (md *Matched) ToString(nu int) string {
	details := []string{}
	for _, m := range md.matches {
		details = append(details, fmt.Sprintf("%d", len(m.Units)))
	}
	return fmt.Sprintf("%d topologies + %d units = %v matches", len(md.invocation.Topologies), nu, details)
}

// Matchmaker returns [n][_]*unit containing plausible matches
// n: index corresponding to the invocation topologies
// _: if all=false, len is 0 (nomatch) or 1 (match). if all=true, len is uint
func Matchmaker(units map[string]*unit, inv *invocation, all bool) (*Matched, error) {
	topos := inv.Topologies
	md := NewMatched(inv)
	matches := 0
	for nt, t := range topos { // yes, premature bundle code, but doesn't seem to hurt
		m := &Match{Topology: t, Units: []*unit{}}
		md.matches[nt] = m
		// newBitmap := bitmap.Bitmap{} probably https://github.com/RoaringBitmap/roaring
		for _, u := range units {
			// if unit is taken, skip
			if false == all && u.IsAllocated() {
				continue
			}
			// TODO: make the topology a struct so I can abstract matching away
			if u.Topology.GetName() == t.GetName() { // PROTOTYPE ONLY
				// set bit _ in the bitmap
				m.Units = append(m.Units, u)
				if !all { // simplify: first match for speed
					break
				}
			}
		}
		if len(m.Units) > 0 {
			matches += 1
		}
	}
	// units -> slice, not map
	logger.Go.Infof("%v results: %s", inv.ID, md.ToString(len(units)))
	// maybe we need a matchmaker struct
	return md, nil
}

// Allocate validates invocation request is satisfiable, then queues it.
// See the proto docstrings for more details.
func (s *Service) Allocate(ctx context.Context, req *apb.AllocateRequest) (retRes *apb.AllocateResponse, retErr error) {
	defer updateMetrics("Allocate", &retErr, timeNow())
	s.mu.Lock()
	defer s.mu.Unlock()
	invMsg := req.GetInvocation()
	if len(invMsg.GetTopologies()) != 1 {
		return nil, status.Errorf(codes.InvalidArgument, "requests must have exactly one topology (for now)")
	}
	inv := &invocation{Topologies: req.Invocation.GetTopologies()} // Matchmaker only uses the topos
	invocationID := invMsg.GetId()
	// Enqueue it
	if invocationID == "" {
		// only check first time:
		matches, err := Matchmaker(s.units, inv, true)
		if err != nil {
			return nil, err
		}
		if !matches.Found() {
			// TODO make error more verbose
			return nil, status.Errorf(codes.InvalidArgument, "results: %s . "+
				" impossible to match against inventory. This is a permanent failure, not"+
				" an availability failure.", matches.ToString(len(s.units)))
		}
		// This is the first AllocationRequest. Generate an ID and queue it.
		invocationID, err = generateRandomID()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to generate invocation_id: %v", err)
		}
		inv := &invocation{
			ID:          invocationID,
			Owner:       invMsg.GetOwner(),
			Purpose:     invMsg.GetPurpose(),
			LastCheckin: timeNow(),
			Topologies:  invMsg.GetTopologies(),
		}
		InvocationQueue.Enqueue(inv)
		if s.currentState == stateRunning {
			InvocationQueue.Promote(s.units) // run asap so we can tell the user whether they're allocated or queued below
		}
	}
	// Update LastCheckin
	topos := []*apb.Topology{}
	for _, u := range s.units {
		if inv := u.GetInvocation(invocationID); inv != nil {
			inv.LastCheckin = timeNow()
			topos = append(topos, &u.Topology)
			break // TODO: for bundles, remove this break
		}
	}
	// Invocation was already allocated (i.e. by janitor())
	if len(topos) > 0 {
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
	// Invocation is queued
	if inv, pos := InvocationQueue.Get(invocationID); inv != nil {
		inv.LastCheckin = timeNow()
		logger.Go.Infof("Queued(%s)", invocationID)
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
	// Invocation is neither allocated nor queued
	if s.currentState == stateRunning {
		// This invocation is unknown (possibly expired)
		return nil, status.Errorf(codes.FailedPrecondition, "invocation_id not found: %q", invocationID)
	}
	// This invocation was previously queued before the server restart; add it
	// back to the queue.
	inv = &invocation{
		ID:          invocationID,
		Owner:       invMsg.GetOwner(),
		Purpose:     invMsg.GetPurpose(),
		LastCheckin: timeNow(),
		Topologies:  invMsg.GetTopologies(),
	}
	pos := InvocationQueue.Enqueue(inv)
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
