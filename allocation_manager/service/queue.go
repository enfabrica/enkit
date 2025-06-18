package service

import (
	"sort"
	"time"

	apb "github.com/enfabrica/enkit/allocation_manager/proto"

	"github.com/enfabrica/enkit/lib/logger"
)

// invocationQueue is a simple queue of invocations.
//
// Beside implementing the classic Enqueue and Dequeue methods:
//   - It partially implements the sort.Interface to allow the easy
//     implementation of invocation prioritization policies.
//   - It maintains per-entry state (QueueID) so it's easy to tell
//     the position of each invocation by just looking at that value.
//
// invocationQueue is NOT thread safe. The caller must ensure that
// only a single thread can access the queue at any given time.
type invocationQueue []*invocation

// all requests go into a global queue
var InvocationQueue invocationQueue

// Position represents a relative position within the queue.
//
// For example, if this is the 2rd element in the queue, Position will be 3.
//
// Given the QueueID of an element its Position can be computed in O(0) by
// subtracting the QueueID of the first element currently in the queue.
type Position uint32

// QueueID is a monotonically increasing number representing the absolute
// position of the item in the queue from when the queue was last emptied.
//
// If the element is reordered, so to be dequeued earlier, its QueueID
// will be changed accordingly.
type QueueID uint64

// Enqueue adds an item to the queue.
//
// During addition, a QueueID is assigned. The invocation is placed at the back of the queue.
//
// Returns the 1-based index the invocation was queued at.
func (iq *invocationQueue) Enqueue(x *invocation) Position {
	// TODO: metrics
	// defer iq.updateMetrics()
	// TODO: prioritizer
	// iq.prioritizer.OnEnqueue(x)
	// u.queue.Sort(u.prioritizer.Sorter())
	position := Position(len(*iq))
	offset := QueueID(1)
	if len(*iq) > 0 {
		offset = (*iq)[0].QueueID
	}
	x.QueueID = offset + QueueID(position)
	(*iq) = append(*iq, x)
	return position
}

// Promote tries to turn queued requests into allocations.
func (iq *invocationQueue) Promote(units map[string]*Unit, inventory *apb.HostInventory, topologies map[string]*Topology) {
	// TODO: metrics
	// defer iq.updateMetrics()
	for _, inv := range *iq {
		matches, err := Matchmaker(units, inventory, topologies, inv, false)
		if err != nil {
			logger.Go.Warnf("Promote() is ignoring Matchmaker err=%v\n", err)
			continue // short circuit
		}
		// TODO: optimize across the matches x requests matrix after bitmap implemented
		if len(matches) > 0 {
			iq.Forget(inv.ID) // dequeue
			// associate units with invocation
			for _, match := range matches {
				// Matchmaker should return one matching topology
				if match.Topology == nil {
					logger.Go.Errorf("Error: Match has no topology")
					break
				}
				
				if !match.Topology.Allocate(inv) {
					logger.Go.Errorf("Topology Allocate not supposed to fail! match=%v\n", match)
				}
			}
		}
		// TODO: prioritizer
		// u.prioritizer.OnDequeue(invocation)
		// u.prioritizer.OnAllocate(invocation)
	}
}

// Dequeue removes an item from the queue.
//
// When an item is removed from the queue, its QueueID is reset to zero.
func (iq *invocationQueue) Dequeue() *invocation {
	if len(*iq) <= 0 {
		return nil
	}
	retval := (*iq)[0]
	(*iq) = (*iq)[1:len(*iq)]
	retval.QueueID = 0
	return retval
}

// Len returns the length of the queue.
func (iq *invocationQueue) Len() int {
	return len(*iq)
}

// Swap swaps two elements of the queue, taking care of updating the QueueID.
func (iq *invocationQueue) Swap(i, j int) {
	(*iq)[i].QueueID, (*iq)[j].QueueID = (*iq)[j].QueueID, (*iq)[i].QueueID
	(*iq)[i], (*iq)[j] = (*iq)[j], (*iq)[i]
}

// Filter is a function used to remove entries from the queue.
//
// It is invoked once per invocation in the queue, returns true if the entry
// has to be filtered (removed), false if the entry has to be kept.
type Filter func(pos Position, inv *invocation) bool

// Walker is a function that does something for each entry in the queue.
//
// It is invoked once per invocation in the queue, returns true if the walk
// should continue, false if it should stop.
type Walker func(pos Position, inv *invocation) bool

// Walk invokes the walker function for each element of the queue.
//
// If the walker returns false, the walk is interrupted.
//
// Returns the invocation and position of the element the walk was
// interrupted on, or (nil, 0).
func (iq *invocationQueue) Walk(walker Walker) (*invocation, Position) {
	for posx, inv := range *iq {
		pos := Position(posx + 1)
		if !walker(pos, inv) {
			return inv, pos
		}
	}
	return nil, 0
}

// Get returns the invocation and 1-based index position for an invocation id.
// If the invocation id is not found, (nil, 0) is returned instead.
func (iq *invocationQueue) Get(invID string) (*invocation, Position) {
	return iq.Walk(func(pos Position, inv *invocation) bool {
		return inv.ID != invID
	})
}

// Position returns the Position of an invocation in the queue.
//
// If the invocation is not queued, returns the 0 Position.
func (iq *invocationQueue) Position(inv *invocation) Position {
	if len(*iq) <= 0 || inv.QueueID == 0 {
		return 0
	}
	return Position(1 + inv.QueueID - (*iq)[0].QueueID)
}

// Filter removes the elements based on the filter callback provided.
//
// If the filter returns true, the element is removed, otherwise it is preserved.
// Updates the QueueID of elements as they are filtered out/removed from the queue.
//
// Returns the number of elements removed.
func (iq *invocationQueue) Filter(filter Filter) int {
	newQueue := []*invocation{}
	count := 0
	for pos, inv := range *iq {
		if filter(Position(pos+1), inv) {
			count += 1
			continue
		}
		inv.QueueID -= QueueID(count)
		newQueue = append(newQueue, inv)
	}
	(*iq) = newQueue
	return count
}

// Forget removes the specified invocation ID from the queue.
//
// Returns the invocation removed, or nil if it was not found.
// As the element is removed, QueueID is reset to 0.
func (iq *invocationQueue) Forget(invID string) *invocation {
	var retinv *invocation
	iq.Filter(func(pos Position, inv *invocation) bool {
		if inv.ID == invID {
			retinv = inv
			retinv.QueueID = 0 // It is no longer queued.
			return true
		}
		return false
	})
	return retinv
}

// ExpireQueued removes all queued invocations that have not checked in since
// `expiry`.
func (iq *invocationQueue) ExpireQueued(expiry time.Time) {
	// TODO: metrics
	// defer iq.updateMetrics()
	iq.Filter(func(pos Position, inv *invocation) bool {
		if inv.LastCheckin.After(expiry) {
			return false
		}
		// TODO: prioritizer
		// iq.prioritizer.OnDequeue(inv)
		// TODO: metrics
		// metricReleaseReason.WithLabelValues("queued_expired").Inc()
		return true
	})
}

// Sorter is a function capable of prioritizing an invocation over another.
//
// It behaves like the Less() function in the Sort.Interface definition,
// moving "lesser" items to the front of the queue.
type Sorter func(a, b *invocation) bool

// Used with sort.Sort().
type sorter struct {
	*invocationQueue
	sort Sorter
}

// Less implements the Sort.Interface Less method.
func (s *sorter) Less(i, j int) bool {
	return s.sort((*s.invocationQueue)[i], (*s.invocationQueue)[j])
}

// Sort sorts the queue using the supplied Sorter.
func (iq *invocationQueue) Sort(p Sorter) {
	if p == nil {
		return
	}
	sort.Stable(&sorter{invocationQueue: iq, sort: p})
}
