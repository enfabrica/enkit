package service

// Prioritizer is an object capable of sorting a queue in order of priority.
//
// In order for the prioritizer to have enough data to make the prioritization
// decision, it must be invoked every time an entry is queued and dequeued, and
// every time a license is allocated and released.
type Prioritizer interface {
	// OnEnqueue is called every time an invocation is queued.
	OnEnqueue(inv *invocation)
	// OnDequeue is called every time an invocation is dequeued.
	//
	// Note that an invocation may be dequeued because it is expired,
	// because the user withdrew the request, or because an allocation
	// is now possible.
	// In this last case, OnDequeue() will be followed by an OnAllocate()
	// call.
	OnDequeue(inv *invocation)

	// OnAllocate is called every time an invocation is allocated a license.
	OnAllocate(inv *invocation)
	// OnRelease is called every time an invocation loses a license.
	//
	// This generally means that the invocation was serviced and the
	// license is no longer needed, but it could be called also if,
	// for example, a client timed out while holding a license.
	OnRelease(inv *invocation)

	// Sorter is a function that returns a function capable of
	// reordering the queue.
	//
	// The returned Sorter is generally passed to invocationQueue.Sort.
	Sorter() Sorter
}

// FIFOPrioritizer simply keeps the entries in the order they were queued.
type FIFOPrioritizer struct {
}

func (_ *FIFOPrioritizer) OnEnqueue(inv *invocation) {
}
func (_ *FIFOPrioritizer) OnDequeue(inv *invocation) {
}
func (_ *FIFOPrioritizer) OnAllocate(inv *invocation) {
}
func (_ *FIFOPrioritizer) OnRelease(inv *invocation) {
}
func (_ *FIFOPrioritizer) Sorter() Sorter {
	return nil
}

// EvenOwnerAllocationsPrioritizer is a prioritizer that tries to spread
// license allocations evenly across owners.
//
// Time is not part of the equation: if user A has 10 licenses allocated
// and an user B comes by, it will prioritize B over A until there is a
// 5 ~ 5 distribution in number of licenses.
type EvenOwnersPrioritizer struct {
	// Key: invocation.ID, value represents the absolute position of
	// the element in the queue for the specific owner.
	position map[string]uint64

	// Key: invocation.Owner, value represents # of requests queued,
	// monotonically increasing.
	enqueued map[string]uint64
	// Key: invocation.Owner, value represents # of requests dequeued,
	// monotonically increasing.
	//
	// "dequeued - queued" provides the # of entries in queue for the user.
	// "current queue id - dequeued" provides the position in the queue.
	dequeued map[string]uint64

	// Key: invocation.Owner, value represents # of licenses allocated
	// for the user.
	allocated map[string]uint64
}

func NewEvenOwnersPrioritizer() *EvenOwnersPrioritizer {
	return &EvenOwnersPrioritizer{
		position:  map[string]uint64{},
		enqueued:  map[string]uint64{},
		dequeued:  map[string]uint64{},
		allocated: map[string]uint64{},
	}
}

func (so *EvenOwnersPrioritizer) OnEnqueue(inv *invocation) {
	so.enqueued[inv.Owner] += 1
	so.position[inv.ID] = so.enqueued[inv.Owner]
}

func (so *EvenOwnersPrioritizer) OnDequeue(inv *invocation) {
	so.dequeued[inv.Owner] += 1
	delete(so.position, inv.ID)
	if so.dequeued[inv.Owner]-so.enqueued[inv.Owner] <= 0 {
		delete(so.enqueued, inv.Owner)
		delete(so.dequeued, inv.Owner)
		return
	}
}

func (so *EvenOwnersPrioritizer) OnAllocate(inv *invocation) {
	so.allocated[inv.Owner] += 1
}

func (so *EvenOwnersPrioritizer) OnRelease(inv *invocation) {
	value := so.allocated[inv.Owner]
	if value <= 1 {
		delete(so.allocated, inv.Owner)
		return
	}
	so.allocated[inv.Owner] = value - 1
}

func (so *EvenOwnersPrioritizer) Sorter() Sorter {
	return func(a, b *invocation) bool {
		ap := so.position[a.ID]
		am := so.dequeued[a.Owner]
		aa := so.allocated[a.Owner]

		bp := so.position[b.ID]
		bm := so.dequeued[b.Owner]
		ba := so.allocated[b.Owner]

		// To explain this code, let's start from some definitions:
		//
		// uqp = (Priority - dequeued) = represents a "per user queue
		//   position". For user "foo", uqp 0 is the first entry, uqp
		//   1 is the second entry, uqp 2 is the third entry...
		//   no matter the real position of the entry in the global
		//   queue. Sorting a global queue by the "uqp" of each user
		//   will ensure that invocations are allocated evenly by user,
		//   as the queue will alternate between users.
		//
		// uqa = uqp + allocations = gives us a number we can use so
		//   users with fewer entries in the queue are prioritized first.
		//
		// Example:
		//   OnAllocate: User A=10, User B=0
		//
		//          Queue is:  A  A  A  A  A  B  A  A  B  B  B  B  B  B
		//               uqp:  0  1  2  3  4  0  5  6  1  2  3  4  5  6
		//               uqa: 10 11 12 13 14  0  15 16 1  2  3  4  5  6
		//
		// -> After sorting by uqa, all requests from B will be served first.
		//
		// Let's say A and B queue more requests, and a 3rd user comes by:
		//
		//   OnAllocate: User A=10, User B=6, User C=0
		//          Queue is:  A  A  A  A  A  A  A  B  B  B  B  B  C  C
		//               uqp:  0  1  2  3  4  5  6  0  1  2  3  4  0  1
		//               uqa: 10 11 12 13 14 15 16  6  7  8  9 10  0  1
		//
		// -> C will take precendence. Then B will queue 4 more licenses.
		//    A and B at that point will be even and start competing
		//    equally for the next slots, with C still taking priority.
		return (ap - am + aa) < (bp - bm + ba)
	}
}
