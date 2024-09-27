package service

// TODO(kjw): separate packages?

import (
	"strings"
	"time"
	"fmt"

	apb "github.com/enfabrica/enkit/allocation_manager/proto"
	"github.com/enfabrica/enkit/lib/logger"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/prometheus/client_golang/prometheus"
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

unitCounter = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "unit_operations_total",
		Help: "Total number of operations performed on units",
	},
	[]string{"kind", "unit", "operation"},
)
)

func init() {
	fmt.Println("In init")
	prometheus.MustRegister(unitCounter)
}

func (u unit) DoOperation(operationName string) {
	// name := u.name
	name := u.Topology.Name
	unitCounter.With(prometheus.Labels{
		"kind": fmt.Sprintf("%T", u),
		"unit": name,
		"operation": operationName}).Inc()
	fmt.Printf("Operation '%s' performed on unit: %s %T\n", operationName, name, u)
	// fmt.Println("Counter:", unitCounter.Counter)
}

type unit struct { // store topologies describing actual hardware
	Health     apb.Health   // Health status of hardware
	Topology   apb.Topology // hardware actual configuration
	Invocation *invocation  // request for a Unit allocation
}

func newUnit(topo apb.Topology) *unit {
	u := new(unit)
	u.Topology = topo
	fmt.Println("New unit allocated", topo)
	return u
}

// Allocate attempts to associate the supplied invocation with this Unit.
// Returns whether this Unit was successfully allocated.
func (u *unit) Allocate(inv *invocation) bool {
	defer u.updateMetrics()
	if u.Invocation != nil && u.Invocation.ID == inv.ID {
		return false
	}
	// u.prioritizer.OnAllocate(inv)
	logger.Go.Infof("unit.Allocate %s to %s\n", u.Topology.Name, inv.ID)
	u.Invocation = inv
	return true
}

func (u *unit) IsAllocated() bool {
	return nil != u.Invocation
}

// TODO: decide actual values to go in here
func (u *unit) IsHealthy() bool {
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
func (u *unit) GetInvocation(invID string) *invocation {
	if u.Invocation != nil && u.Invocation.ID == invID {
		return u.Invocation
	}
	return nil
}

// ExpireAllocations removes all allocations for invocations that have not
// checked in since `expiry`.
func (u *unit) ExpireAllocations(expiry time.Time) {
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
func (u *unit) Forget(invID string) int {
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
func (u *unit) GetStats() *apb.Stats {
	fields := strings.SplitN(u.Topology.GetName(), "::", 2)
	if len(fields) != 2 {
		fields = []string{"<UNKNOWN>", u.Topology.GetName()}
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
		Topology:  &u.Topology,
		Status:    status,
		Timestamp: timestamppb.New(timeNow()),
	}
}

func (u *unit) updateMetrics() {
	// metricActiveCount.WithLabelValues(u.name).Set(float64(len(u.allocations)))
	// metricQueueSize.WithLabelValues(u.name).Set(float64(u.queue.Len()))
	// metricTotalLicenses.WithLabelValues(u.name).Set(float64(u.totalAvailable))
}
