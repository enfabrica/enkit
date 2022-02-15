package service

import (
	"sort"
)

// invocationQueue is a simple queue of invocations.
//
// Beside implementing the classic Enqueue and Dequeue methods:
// - It partially implements the sort.Interface to allow the easy
//   implementation of invocation prioritization policies.
// - It maintains per-entry state (QueueID) so it's easy to tell
//   the position of each invocation by just looking at that value.
type invocationQueue []*invocation

func (iq *invocationQueue) Enqueue(x *invocation) Position {
	position := Position(len(*iq))
	offset := QueueID(1)
	if len(*iq) > 0 {
		offset = (*iq)[0].QueueID
	}

	x.QueueID = offset + QueueID(position)
	(*iq) = append(*iq, x)
	return position
}

func (iq *invocationQueue) Dequeue() *invocation {
	if len(*iq) <= 0 {
		return nil
	}

	retval := (*iq)[0]
	(*iq) = (*iq)[1:len(*iq)]
	retval.QueueID = 0
	return retval
}

func (iq *invocationQueue) Len() int {
	return len(*iq)
}

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

func (iq *invocationQueue) Walk(walker Walker) (*invocation, Position) {
	for posx, inv := range *iq {
		pos := Position(posx + 1)
		if !walker(pos, inv) {
			return inv, pos
		}
	}
	return nil, 0
}

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

func (iq *invocationQueue) Get(invID string) (*invocation, Position) {
	return iq.Walk(func(pos Position, inv *invocation) bool {
		return inv.ID != invID
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

// Position returns the Position of an invocation in the queue.
// If the invocation is not queued, returns the 0 Position.
func (iq *invocationQueue) Position(inv *invocation) Position {
	if len(*iq) <= 0 {
		return 0
	}

	return Position(1 + inv.QueueID - (*iq)[0].QueueID)
}
