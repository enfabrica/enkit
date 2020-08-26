package scheduler

import (
	"github.com/stretchr/testify/assert"
	"sync/atomic"
	"testing"
	"time"
)

func TestScheduler(t *testing.T) {
	all := []time.Duration{}

	// Why dedup and the > 1 second logic? The loop in the scheduler is racy by design.
	//
	// When a new event is scheduled, the current timer is interrupted, and a new timer
	// is computed based on the time of the newly inserted event.
	// Depending on execution speed, it is not known if this newly scheduled timer
	// will hit before other timers have been started, and which.
	//
	// Further, every "WithTimeWait" interval, the scheduler will interrupt the timers and
	// calculate new ones periodically.
	//
	// The code here tries to avoid recording those spurious, expected, events in dedup,
	// while using 'all' to verify that we expected more of the minimum amount of events,
	// and less than the maximum.
	dedup := []time.Duration{}
	// A fake time after source that will always make it look like the timer expired.
	ta := func(d time.Duration) <-chan time.Time {
		all = append(all, d)
		t := make(chan time.Time, 1)
		if d > 1*time.Second {
			return t
		}

		if len(dedup) <= 0 || dedup[len(dedup)-1] != d {
			dedup = append(dedup, d)
		}
		t <- time.Unix(0, 0)
		return t
	}
	// A fake time source that will always make it look like no time has passed.
	ts := func() time.Time {
		return time.Unix(0, 0)
	}

	check := 1 * time.Minute
	s := New(WithTimeSource(ts), WithTimeAfter(ta), WithTimeWait(check))

	called := int32(0)
	inc := func() {
		atomic.AddInt32(&called, 1)
	}

	s.AddAfter(50*time.Millisecond, inc)
	s.AddAfter(200*time.Millisecond, inc)
	s.AddAt(time.Unix(0, 300000000), inc)
	s.Wait()

	assert.Equal(t, int32(3), atomic.LoadInt32(&called))
	assert.LessOrEqual(t, 6, len(all))
	assert.GreaterOrEqual(t, 10, len(all))
	assert.Equal(t, []time.Duration{50 * time.Millisecond, 200 * time.Millisecond, 300 * time.Millisecond}, dedup)
	s.Done()
}
