package utils

import (
	"time"
)

// TimeSource is a function returning the current time.
//
// This is useful to allow mocking time with functions using time.Now().
type TimeSource func() time.Time

// Clock is an object capable of returning time.
//
// This is useful to allow mocking time with functions using time.After()
// as well as time.Now().
type Clock interface {
	Now() time.Time
	After(d time.Duration) <-chan time.Time
}

// SystemClock is a Clock that returns the real system time.
type SystemClock struct {
}

func (SystemClock) Now() time.Time {
	return time.Now()
}

func (SystemClock) After(d time.Duration) <-chan time.Time {
	return time.After(d)
}
