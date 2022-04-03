package utils

import (
	"sync/atomic"
	"time"
)

// AtomicTime represents a unix time in nanoseconds.
//
// It provides methods to access and update this time atomically.
type AtomicTime int64

// Set will set the atomic time to the value supplied.
func (at *AtomicTime) Set(t time.Time) {
	nano := t.UnixNano()
	atomic.StoreInt64((*int64)(at), nano)
}

// Reset will reset the time to 0.
//
// Note that 0 is actually a valid time: January 1st, 1970.
// This may or may not be ok depending on the use of AtomicTime.
func (at *AtomicTime) Reset() {
	atomic.StoreInt64((*int64)(at), 0)
}

// Returns the AtomicTime as unix time in nanoseconds.
//
// The value returned is comparable with time.Now().UnixNano().
func (at *AtomicTime) Nano() int64 {
	return atomic.LoadInt64((*int64)(at))
}
