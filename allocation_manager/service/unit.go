package service

// TODO(kjw): separate packages?

import (
	"strings"
	"time"

	apb "github.com/enfabrica/enkit/allocation_manager/proto"

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

type unit struct { // store topologies describing actual hardware
	Health     apb.Health   // Health status of hardware
	Topology   apb.Topology // hardware actual configuration
	Invocation *invocation  // request for a Unit allocation
}

/* TODO: move queue out
// Enqueue puts the supplied invocation at the back of the queue. Returns the
// 1-based index the invocation was queued at.
func (u *unit) Enqueue(inv *invocation) Position {
	defer u.updateMetrics()
	u.queue.Enqueue(inv)
	u.prioritizer.OnEnqueue(inv)
	u.queue.Sort(u.prioritizer.Sorter())
	return u.queue.Position(inv)
}
*/

// Allocate attempts to associate the supplied invocation with this Unit.
// Returns whether this Unit was successfully allocated.
func (u *unit) Allocate(inv *invocation) bool {
	defer u.updateMetrics()
	if u.Invocation != nil && u.Invocation.ID == inv.ID {
		return false
	}
	// u.prioritizer.OnAllocate(inv)
	u.Invocation = inv
	return true
}

// TODO: move queue out
//// Promote attempts to promote queued requests to allocations until either no
//// licenses remain or no queued requests remain.
//func (u *unit) Promote() {
//	defer u.updateMetrics()
//	numFree := u.totalAvailable - len(u.allocations)
//	for i := 0; i < numFree && u.queue.Len() > 0; i++ {
//		u.queue.Sort(u.prioritizer.Sorter())
//		invocation := u.queue.Dequeue()
//		u.prioritizer.OnDequeue(invocation)
//		u.prioritizer.OnAllocate(invocation)
//		u.allocations[invocation.ID] = invocation
//	}
//}

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
		u.Invocation = nil
		// move health to unknown?
	}
}

/* TODO: Move into queue
// ExpireQueued removes all queued invocations that have not checked in since
// `expiry`.
func (u *unit) ExpireQueued(expiry time.Time) {
	defer u.updateMetrics()
	u.queue.Filter(func(pos Position, inv *invocation) bool {
		if inv.LastCheckin.After(expiry) {
			return false
		}
		u.prioritizer.OnDequeue(inv)
		metricLicenseReleaseReason.WithLabelValues("queued_expired").Inc()
		return true
	})
}

// GetQueued returns an invocation by ID if the invocation is queued, or nil
// otherwise. If the returned invocation is not nil, the 1-based index (queue
// position) is also returned.
func (u *unit) GetQueued(invID string) (*invocation, Position) {
	inv, pos := u.queue.Get(invID)
	return inv, pos
}
*/

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

// Forget removes invocations matching the specified ID from allocations and
// the queue.
func (u *unit) Forget(invID string) int {
	defer u.updateMetrics()
	count := 0
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
		u.Invocation = nil
	}
	return count
}

func (u *unit) updateMetrics() {
	// metricActiveCount.WithLabelValues(u.name).Set(float64(len(u.allocations)))
	// metricQueueSize.WithLabelValues(u.name).Set(float64(u.queue.Len()))
	// metricTotalLicenses.WithLabelValues(u.name).Set(float64(u.totalAvailable))
}
