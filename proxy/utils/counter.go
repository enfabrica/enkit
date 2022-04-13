package utils

import (
	"sync/atomic"
)

// Counter is a wrapper around an uint64 using atomic operations to update it.
type Counter uint64

// Increment increments the counter by 1.
func (c *Counter) Increment() {
	atomic.AddUint64((*uint64)(c), 1)
}

// Add increments the counter by the value specified.
func (c *Counter) Add(value uint64) {
	atomic.AddUint64((*uint64)(c), value)
}

// Get returns the value of the counter.
func (c *Counter) Get() uint64 {
	return atomic.LoadUint64((*uint64)(c))
}

// SetIfGreatest saves value in the counter if it is the greates value seen so far.
func (c *Counter) SetIfGreatest(value uint64) {
	// This loop will break once the value is either successfully swapped
	// because it is the greatest, or if it's no longer the greatest.
	for {
		current := atomic.LoadUint64((*uint64)(c))
		if value <= current {
			break
		}

		if atomic.CompareAndSwapUint64((*uint64)(c), current, value) {
			return
		}
	}
}
