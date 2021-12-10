package workpool

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/khttp/scheduler"
	"github.com/enfabrica/enkit/lib/retry"
	"github.com/stretchr/testify/assert"
	"log"
	"sync"
	"testing"
	"time"
)

func TestResult(t *testing.T) {
	wp, err := New(WithWorkers(5), WithQueueSize(10), WithImmediateQueueSize(5))
	assert.Nil(t, err)

	results := []*Result{}
	for i := 0; i < 100; i++ {
		result := ResultRetriever()

		ix := i
		hello := func() interface{} {
			return fmt.Sprintf("hello, human %d", ix)
		}

		wp.Add(WithResult(hello, result))
		results = append(results, result)
	}
	for i, result := range results {
		assert.Equal(t, fmt.Sprintf("hello, human %d", i), result.Get().(string))
	}
}

func TestCallback(t *testing.T) {
	wp, err := New(WithWorkers(5), WithQueueSize(10), WithImmediateQueueSize(5))
	assert.Nil(t, err)

	results := []*Result{}
	for i := 0; i < 100; i++ {
		result := ResultRetriever()

		ix := i
		hello := func() interface{} {
			return fmt.Sprintf("hello, human %d", ix)
		}

		wp.Add(WithResult(hello, result))
		results = append(results, result)
	}
	for i, result := range results {
		assert.Equal(t, fmt.Sprintf("hello, human %d", i), result.Get().(string))
	}
}

func TestRetry(t *testing.T) {
	// A fake time after source that will always make it look like the timer expired.
	waits := 0
	ta := func(d time.Duration) <-chan time.Time {
		t := make(chan time.Time, 1)
		if d > 1*time.Second {
			return t
		}
		log.Printf("WAIT %s", d)
		if d == 200*time.Millisecond {
			waits += 1
		}
		t <- time.Unix(0, 0)
		return t
	}
	// A fake time source that will always make it look like no time has passed.
	ts := func() time.Time {
		return time.Time{}
	}

	check := 1 * time.Minute
	wg := &sync.WaitGroup{}
	s := scheduler.New(scheduler.WithTimeSource(ts), scheduler.WithTimeAfter(ta), scheduler.WithTimeWait(check), scheduler.WithWaitGroup(wg))
	wp, err := New(WithWorkers(5), WithQueueSize(10), WithImmediateQueueSize(5), WithWaitGroup(wg))
	assert.Nil(t, err)

	success := 0
	calls := 0

	// Fail a couple times, than succeed.
	options := retry.New(retry.WithTimeSource(ts), retry.WithWait(200*time.Millisecond), retry.WithAttempts(5), retry.WithFuzzy(0))
	wp.Add(WithRetry(options, s, wp, func() error {
		calls += 1
		if calls < 3 {
			return fmt.Errorf("error attempt %d", calls)
		}
		success += 1
		return nil
	}, ErrorIgnore))
	wg.Wait()
	assert.Equal(t, 3, calls)
	assert.Equal(t, 1, success)
	assert.Equal(t, 2, waits)

	// Never succeed, should eventually give up..
	var errs error
	calls = 0
	waits = 0
	options = retry.New(retry.WithTimeSource(ts), retry.WithWait(200*time.Millisecond), retry.WithAttempts(5), retry.WithFuzzy(0))
	wp.Add(WithRetry(options, s, wp, func() error {
		calls += 1
		return fmt.Errorf("error attempt %d", calls)
	}, ErrorCallback(func(err error) { errs = err })))
	wg.Wait()

	assert.NotNil(t, errs)
	assert.Equal(t, "Multiple errors:\n  error attempt 1\n  error attempt 2\n  error attempt 3\n  error attempt 4\n  error attempt 5", errs.Error())
	assert.Equal(t, 5, calls)
	assert.Equal(t, 4, waits) // No wait after the last call, and before the first.
}
