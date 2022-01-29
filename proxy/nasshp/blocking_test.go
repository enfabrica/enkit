package nasshp

import (
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
	"time"
)

func ExampleWaiter() {
	lock := &sync.Mutex{}
	waiter := NewWaiter(lock)
	wg := &sync.WaitGroup{}
	wg.Add(2)

	counter := 0
	// Produces events: increments counter.
	go func() {
		for i := 0; i < 10; i++ {
			time.Sleep(10 * time.Millisecond)
			lock.Lock()
			counter += 1
			waiter.Signal()
			lock.Unlock()
		}
		wg.Done()
	}()

	go func() {
		lock.Lock()
		defer lock.Unlock()
		for counter < 10 {
			waiter.Wait()
		}
		fmt.Println("counter is now >= 10")
		wg.Done()
	}()
	wg.Wait()
	// Output: counter is now >= 10
}

func TestWaiterFail(t *testing.T) {
	lock := &sync.Mutex{}
	waiter := NewWaiter(lock)

	// It is safe to fail multiple times.
	// (could happen from separate threads).
	waiter.Fail(errors.New("test error"))
	waiter.Fail(errors.New("second error"))
}

func TestWaiterBasic(t *testing.T) {
	lock := &sync.Mutex{}
	waiter := NewWaiter(lock)
	counter := 0

	go func() {
		for i := 0; i < 10; i++ {
			time.Sleep(50 * time.Millisecond)

			lock.Lock()
			counter += 1
			lock.Unlock()

			waiter.Signal()
		}
	}()

	lock.Lock()
	defer lock.Unlock()
	for counter < 10 {
		e := waiter.Wait()
		assert.NoError(t, e)
	}
	assert.Equal(t, 10, counter)
}
