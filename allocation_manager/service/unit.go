package service

// TODO(kjw): separate packages?

import (
	"strings"
	"time"

	apb "github.com/enfabrica/enkit/allocation_manager/proto"
	"github.com/enfabrica/enkit/lib/logger"

	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
/* TODO queue
metricQueueSize = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Subsystem: "allocation_manager",
	Name:      "queued_invocations",
	Help:      "Number of queued invocations for a particular license",
},
	[]string{
		// The license vendor + feature, in `vendor::feature` format.
		"license_type",
	},
)
*/
/* TODO centralized counts of units
metricTotalLicenses = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Subsystem: "allocation_manager",
	Name:      "total_licenses",
	Help:      "The total number of licenses purchased",
},
	[]string{
		// The license vendor + feature, in `vendor::feature` format.
		"license_type",
	},
)
metricLicenseReleaseReason = promauto.NewCounterVec(prometheus.CounterOpts{
	Subsystem: "allocation_manager",
	Name:      "license_release_count",
	Help:      "License release count by reason",
},
	[]string{
		"reason",
	},
)
*/
)

type Unit struct { // store topologies describing actual hardware
	Health     	apb.Health   // Health status of hardware
	Invocation 	*invocation  // request for a Unit allocation
	UnitInfo   	apb.UnitInfo
}

func (unit *Unit) GetName() string {
	switch info := unit.UnitInfo.Info.(type) {
	case *apb.UnitInfo_HostInfo:
		return info.HostInfo.GetHostname()
	// TODO: Add case for AcfInfo
	default:
		return "N/A"
	}
}

// Allocate attempts to associate the supplied invocation with this Unit.
// Returns whether this Unit was successfully allocated.
func (u *Unit) Allocate(inv *invocation) bool {
	defer u.updateMetrics()
	if u.Invocation != nil && u.Invocation.ID == inv.ID {
		return false
	}
	// u.prioritizer.OnAllocate(inv)
	logger.Go.Infof("unit.Allocate %s to %s\n", u.GetName(), inv.ID)
	u.Invocation = inv
	return true
}

func (u *Unit) IsAllocated() bool {
	return nil != u.Invocation
}

// TODO: decide actual values to go in here
func (u *Unit) IsHealthy() bool {
	switch u.Health {
	case apb.Health_HEALTH_BROKEN: // maybe later
		return true
	case apb.Health_HEALTH_UNKNOWN: // maybe soon
		return true
	case apb.Health_HEALTH_READY: // yes
		return true
	}
	return false
}

// GetInvocation returns an invocation by ID if the invocation is allocated a
// unit, or nil otherwise.
func (u *Unit) GetInvocation(invID string) *invocation {
	if u.Invocation != nil && u.Invocation.ID == invID {
		return u.Invocation
	}
	return nil
}

// ExpireAllocations removes all allocations for invocations that have not
// checked in since `expiry`.
func (u *Unit) ExpireAllocations(expiry time.Time) {
	defer u.updateMetrics()
	if u.Invocation != nil && !u.Invocation.LastCheckin.After(expiry) {
		// u.prioritizer.OnRelease(v)
		// metricLicenseReleaseReason.WithLabelValues("allocated_expired").Inc()
		logger.Go.Infof("unit.ExpireAllocations %v", u.Invocation.ID)
		u.Invocation = nil
		// move health to unknown?
	}
}

// Forget removes invocations matching the specified ID from allocations and
// the queue.
func (u *Unit) Forget(invID string) int {
	defer u.updateMetrics()
	/*
		newAllocations := map[string]*invocation{}
		for k, v := range u.allocations {
			if k == invID {
				u.prioritizer.OnRelease(v)
				count++
				continue
			}
			newAllocations[k] = v
		}
		if inv := u.queue.Forget(invID); inv != nil {
			u.prioritizer.OnDequeue(inv)
			count += 1
		}
		u.allocations = newAllocations
	*/
	if u.Invocation != nil && invID == u.Invocation.ID {
		logger.Go.Infof("unit.Forget(%v)", u.Invocation.ID)
		u.Invocation = nil
		return 1
	}
	return 0
}

// GetStats returns a Stats message for this Unit.
func (u *Unit) GetStats() *apb.Stats {
	fields := strings.SplitN(u.GetName(), "::", 2)
	if len(fields) != 2 {
		fields = []string{"<UNKNOWN>", u.GetName()}
	}
	/*
		queued := []*apb.Invocation{}
		u.queue.Walk(func(pos Position, inv *invocation) bool {
			queued = append(queued, inv.ToProto())
			return true
		})
	*/
	status := &apb.Status{
		Health: u.Health,
	}
	if u.Invocation != nil {
		status.Allocation = apb.Allocation_ALLOCATION_ALLOCATED
	} else {
		// if ? ... apb.Allocation_ALLOCATION_PENDING_AVAILABLE
		status.Allocation = apb.Allocation_ALLOCATION_AVAILABLE
	}
	return &apb.Stats{
		Info:  		&u.UnitInfo,
		Status:    	status,
		Timestamp: 	timestamppb.New(timeNow()),
	}
}

func (u *Unit) updateMetrics() {
	// metricActiveCount.WithLabelValues(u.name).Set(float64(len(u.allocations)))
	// metricQueueSize.WithLabelValues(u.name).Set(float64(u.queue.Len()))
	// metricTotalLicenses.WithLabelValues(u.name).Set(float64(u.totalAvailable))
}

// a Topology is comprised of one or more units
// TODO: Add links and acfs to Topology
type Topology struct {
	Name	string
	Units	[]*Unit
}

func (topo *Topology) Allocate(inv *invocation) bool {
	for _, unit := range topo.Units {
		if !unit.Allocate(inv) {
			logger.Go.Errorf("Unit Allocate not supposed to fail!")
			return false
		}
	}
	return true
}

func (topo *Topology) CanBeAllocated() bool {
	for _, unit := range topo.Units {
		if unit.IsAllocated() {
			return false
		}
	}
	return true
}
