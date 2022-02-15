package service

import (
	"sort"
	"strings"
	"time"

	fpb "github.com/enfabrica/enkit/flextape/proto"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	metricActiveCount = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: "flextape",
		Name:      "active_invocations",
		Help:      "Number of active invocations for a particular license",
	},
		[]string{
			// The license vendor + feature, in `vendor::feature` format.
			"license_type",
		},
	)
	metricQueueSize = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: "flextape",
		Name:      "queued_invocations",
		Help:      "Number of queued invocations for a particular license",
	},
		[]string{
			// The license vendor + feature, in `vendor::feature` format.
			"license_type",
		},
	)
	metricTotalLicenses = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: "flextape",
		Name:      "total_licenses",
		Help:      "The total number of licenses purchased",
	},
		[]string{
			// The license vendor + feature, in `vendor::feature` format.
			"license_type",
		},
	)
)

// license manages allocations and queued invocations for a single license type.
type license struct {
	name           string                 // Name of the license, in vendor::feature format
	totalAvailable int                    // Constant total number of licenses available for invocations.
	allocations    map[string]*invocation // Map of invocation ID to invocation data for an allocated license.

	queue       invocationq // List of invocations waiting for a license, in FIFO order.
	prioritizer Prioritizer
}

// formatLicenseType returns a unique string for a particular vendor/feature
// combination.
func formatLicenseType(l *fpb.License) string {
	return strings.Join([]string{l.GetVendor(), l.GetFeature()}, "::")
}

// Enqueue puts the supplied invocation at the back of the queue. Returns the
// 1-based index the invocation was queued at.
func (l *license) Enqueue(inv *invocation) Position {
	defer l.updateMetrics()

	l.queue.Enqueue(inv)
	l.prioritizer.OnEnqueue(inv)

	l.queue.Sort(l.prioritizer.Sorter())
	return l.queue.Position(inv)
}

// Allocate attempts to associate the supplied invocation with a license, if
// one is available. Returns whether a license was successfully allocated.
func (l *license) Allocate(inv *invocation) bool {
	defer l.updateMetrics()
	if len(l.allocations) >= l.totalAvailable {
		return false
	}
	l.prioritizer.OnAllocate(inv)
	l.allocations[inv.ID] = inv
	return true
}

// Promote attempts to promote queued requests to allocations until either no
// licenses remain or no queued requests remain.
func (l *license) Promote() {
	defer l.updateMetrics()
	numFree := l.totalAvailable - len(l.allocations)
	for i := 0; i < numFree && l.queue.Len() > 0; i++ {
		l.queue.Sort(l.prioritizer.Sorter())

		invocation := l.queue.Dequeue()

		l.prioritizer.OnDequeue(invocation)
		l.prioritizer.OnAllocate(invocation)

		l.allocations[invocation.ID] = invocation
	}
}

// GetAllocated returns an invocation by ID if the invocation is allocated a
// license, or nil otherwise.
func (l *license) GetAllocated(invID string) *invocation {
	return l.allocations[invID]
}

// ExpireAllocations removes all allocations for invocations that have not
// checked in since `expiry`.
func (l *license) ExpireAllocations(expiry time.Time) {
	defer l.updateMetrics()
	newAllocations := map[string]*invocation{}
	for k, v := range l.allocations {
		if !v.LastCheckin.After(expiry) {
			l.prioritizer.OnRelease(v)
			continue
		}
		newAllocations[k] = v
	}
	l.allocations = newAllocations
}

// ExpireQueued removes all queued invocations that have not checked in since
// `expiry`.
func (l *license) ExpireQueued(expiry time.Time) {
	defer l.updateMetrics()
	l.queue.Filter(func(pos Position, inv *invocation) bool {
		if inv.LastCheckin.After(expiry) {
			return false
		}

		l.prioritizer.OnDequeue(inv)
		return true
	})
}

// GetQueued returns an invocation by ID if the invocation is queued, or nil
// otherwise. If the returned invocation is not nil, the 1-based index (queue
// position) is also returned.
func (l *license) GetQueued(invID string) (*invocation, Position) {
	inv, pos := l.queue.Get(invID)
	return inv, pos
}

// GetStats returns a LicenseStats message for this license type.
func (l *license) GetStats() *fpb.LicenseStats {
	fields := strings.SplitN(l.name, "::", 2)
	if len(fields) != 2 {
		fields = []string{"<UNKNOWN>", l.name}
	}
	allocated := []*fpb.Invocation{}
	for _, inv := range l.allocations {
		allocated = append(allocated, inv.ToProto())
	}
	sort.Slice(allocated, func(i, j int) bool { return allocated[i].Id < allocated[j].Id })
	queued := []*fpb.Invocation{}
	l.queue.Walk(func(pos Position, inv *invocation) bool {
		queued = append(queued, inv.ToProto())
		return true
	})
	return &fpb.LicenseStats{
		License: &fpb.License{
			Vendor:  fields[0],
			Feature: fields[1],
		},
		Timestamp:            timestamppb.New(timeNow()),
		TotalLicenseCount:    uint32(l.totalAvailable),
		AllocatedCount:       uint32(len(l.allocations)),
		AllocatedInvocations: allocated,
		QueuedCount:          uint32(l.queue.Len()),
		QueuedInvocations:    queued,
	}
}

// Forget removes invocations matching the specified ID from allocations and
// the queue.
func (l *license) Forget(invID string) int {
	defer l.updateMetrics()
	count := 0
	newAllocations := map[string]*invocation{}
	for k, v := range l.allocations {
		if k == invID {
			l.prioritizer.OnRelease(v)
			count++
			continue
		}

		newAllocations[k] = v
	}

	if inv := l.queue.Forget(invID); inv != nil {
		l.prioritizer.OnDequeue(inv)
		count += 1
	}

	l.allocations = newAllocations
	return count
}

func (l *license) updateMetrics() {
	metricActiveCount.WithLabelValues(l.name).Set(float64(len(l.allocations)))
	metricQueueSize.WithLabelValues(l.name).Set(float64(l.queue.Len()))
	metricTotalLicenses.WithLabelValues(l.name).Set(float64(l.totalAvailable))
}
