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
	mu                        sync.Mutex       		// Protects the following members from concurrent access
	currentState              state            		// State of the server
	units                     map[string]*Unit 		// Managed Units key=topology name (string) value=*unit
	inventory				  *apb.HostInventory   	// Inventory of available hosts, loaded from provided file
	topologies				  []*Topology	    	// Known topologies, stored server-side, referenced by request
	queueRefreshDuration      time.Duration    		// Queue entries not refreshed within this duration are expired
	allocationRefreshDuration time.Duration    		// Allocations not refreshed within this duration are expired
}

func UnitsFromInventory(inventory *apb.HostInventory) (map[string]*Unit, error) {
 	hostnames := map[string]bool{}
	units := map[string]*Unit{}

	for hostname, inventoried_host := range inventory.GetHosts() {
		unit := new(Unit)
		unit.Health = apb.Health_HEALTH_UNKNOWN // janitor must mark machine as clean
		unit.UnitInfo.Info = &apb.UnitInfo_HostInfo{HostInfo: inventoried_host} // this is the apb.HostInfo for this inventoried host

		_, ok := hostnames[hostname]
		if ok {
			return units, fmt.Errorf("Duplicate hostname: %s", hostname)
		} else {
			hostnames[hostname] = true
			units[hostname] = unit
		}
	}

	return units, nil
}

func TopologiesFromConfigAndUnits(config *apb.Config, units map[string]*Unit) ([]*Topology, error) {
	topos := []*Topology{}

	for _, topo_config := range config.GetTopologyConfigs() {
		topo_units := []*Unit{}

		for _, hostname := range topo_config.GetHosts() {
			unit, ok := units[hostname]
			if !ok {
				return topos, fmt.Errorf("Hostname '%s' not found in unit inventory", hostname)
			}
			topo_units = append(topo_units, unit)
		}

		topo := &Topology{
			Name: topo_config.GetName(),
			Units: topo_units,
		}

		topos = append(topos, topo)
	}

	return topos, nil
}

func defaultUint32(v, d uint32) uint32 {
	if v == 0 {
		return d
	}
	return v
}

func New(config *apb.Config, inventory *apb.HostInventory) (*Service, error) {
	if config.GetServer() == nil {
		return nil, fmt.Errorf("missing `server` section in config")
	}
	queueRefreshSeconds := defaultUint32(config.GetServer().GetQueueRefreshDurationSeconds(), 15)
	allocationRefreshSeconds := defaultUint32(config.GetServer().GetAllocationRefreshDurationSeconds(), 30)
	janitorIntervalSeconds := defaultUint32(config.GetServer().GetJanitorIntervalSeconds(), 1)
	adoptionDurationSeconds := defaultUint32(config.GetServer().GetAdoptionDurationSeconds(), 45)
	units, err := UnitsFromInventory(inventory)
	if err != nil {
		return nil, err
	}

	// TODO: Build topology objects from topology configs + units
	topologies, err := TopologiesFromConfigAndUnits(config, units)
	if err != nil {
		return nil, err
	}

	// Print known topologies
	logger.Go.Infof("Known Topologies")
	if len(topologies) > 0 {
		for _, topo := range topologies {
			logger.Go.Infof(" - %s", topo.Name)
			if len(topo.Units) > 0 {				
				for _, unit := range topo.Units {
					logger.Go.Infof("   - %s", unit.GetName())
				}
			} else {
				logger.Go.Infof("   * no hosts *")
			}
		}
	} else {
		logger.Go.Infof(" * no topologies configured *")
	}

	service := &Service{
		currentState:              stateStarting,
		units:                     units,
		inventory:				   inventory,
		topologies:				   topologies,
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
	ID          	 string    // Server-generated unique ID
	Owner       	 string    // Client-provided owner
	Purpose     	 string    // Client-provided purpose (CI: send test target)
	LastCheckin 	 time.Time // Time the invocation last had its queue position/allocation refreshed.
	QueueID     	 QueueID   // Position in the queue. 0 means the invocation has not been queued yet.
	TopologyRequest  *apb.TopologyRequest
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
	InvocationQueue.Promote(s.units, s.inventory, s.topologies)
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
	TopologyRequest *apb.TopologyRequest  	// The request we recevied for matching a topology
	Topology *Topology    			  		// The known topology that matched the request
}

// Matchmaker returns [n][_]*unit containing plausible matches
// n: index corresponding to the invocation topologies
// _: if all=false, len is 0 (nomatch) or 1 (match). if all=true, len is uint
func Matchmaker(units map[string]*Unit, inventory *apb.HostInventory, topologies []*Topology, inv *invocation, all bool) ([]Match, error) {
	request := inv.TopologyRequest
	matches := []Match{}

	if request.GetTopologyName() != "" {
		// operating off the name of a known topology. check to see if we have a match for this name in our known topologies
		for _, topology := range topologies {
			if topology.Name == request.GetTopologyName() {
				matches = append(matches, Match{TopologyRequest: request, Topology: topology})
			}
		}

		// for _, unit := range units {
		// 	// if unit is taken, skip
		// 	if false == all && unit.IsAllocated() {
		// 		continue
		// 	}

		// 	if unit.GetName() == request.GetTopologyName() {
		// 		// this unit's topology is a match for the request
		// 		// matches = append(matches, Match{TopologyRequest: request, TopologyConfig: })
		// 		if !all {
		// 			break
		// 		}
		// 	}
		// }
	}

	// maybe we need a matchmaker struct
	return matches, nil
}

// Allocate validates invocation request is satisfiable, then queues it.
// See the proto docstrings for more details.
func (s *Service) Allocate(ctx context.Context, req *apb.AllocateRequest) (retRes *apb.AllocateResponse, retErr error) {
	defer updateMetrics("Allocate", &retErr, timeNow())
	s.mu.Lock()
	defer s.mu.Unlock()
	invMsg := req.GetInvocation()
	invocationID := invMsg.GetId()
	invRequest := invMsg.GetRequest()

	if invRequest.GetTopologyName() == "" && len(invRequest.GetHosts()) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "requests must provide either a topology_name, or one or more hosts")
	}

	inv := &invocation{TopologyRequest: invRequest} // Matchmaker only uses the topos
	// Enqueue it
	if invMsg.GetId() == "" {
		// only check first time:
		matches, err := Matchmaker(s.units, s.inventory, s.topologies, inv, true)
		if err != nil {
			return nil, err
		}
		if len(matches) == 0 {
			// TODO make error more verbose
			return nil, status.Errorf(codes.InvalidArgument, "no results. "+
				" impossible to match against inventory. This is a permanent failure, not"+
				" an availability failure.")
		}
		// This is the first AllocationRequest. Generate an ID and queue it.
		invocationID, err := generateRandomID()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to generate invocation_id: %v", err)
		}
		inv := &invocation{
			ID:          		invocationID,
			Owner:       		invMsg.GetOwner(),
			Purpose:     		invMsg.GetPurpose(),
			LastCheckin: 		timeNow(),
			TopologyRequest:  	invRequest,
		}
		InvocationQueue.Enqueue(inv)
		if s.currentState == stateRunning {
			InvocationQueue.Promote(s.units, s.inventory, s.topologies) // run asap so we can tell the user whether they're allocated or queued below
		}
	}
	// Update LastCheckin
	unit_infos := []*apb.UnitInfo{}
	for _, u := range s.units {
		if inv := u.GetInvocation(invocationID); inv != nil {
			inv.LastCheckin = timeNow()
			unit_infos = append(unit_infos, &u.UnitInfo)
			break // TODO: for bundles, remove this break
		}
	}
	// Invocation was already allocated (i.e. by janitor())
	if len(unit_infos) > 0 {
		return &apb.AllocateResponse{
			ResponseType: &apb.AllocateResponse_Allocated{
				Allocated: &apb.Allocated{
					Id:              invocationID,
					UnitInfos:       unit_infos,
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
	// TODO: add to front of queue
	inv = &invocation{
		ID:          		invocationID,
		Owner:       		invMsg.GetOwner(),
		Purpose:     		invMsg.GetPurpose(),
		LastCheckin: 		timeNow(),
		TopologyRequest:  	invRequest,
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
	topoReq := reqInvoc.GetRequest()
	
	allocated := req.GetAllocated() // repeated Topology	
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
	logger.Go.Infof("Refresh(%s)", invID)
	unitInvoc := u.GetInvocation(invID)
	if unitInvoc == nil {
		if s.currentState == stateRunning {
			return nil, status.Errorf(codes.FailedPrecondition, "invocation_id not allocated: %q", invID)
		}
		// else "Adopt" this invocation
		inv := &invocation{
			ID:         	 invID,
			Owner:      	 reqInvoc.GetOwner(),
			Purpose:    	 reqInvoc.GetPurpose(),
			TopologyRequest: topoReq,
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
