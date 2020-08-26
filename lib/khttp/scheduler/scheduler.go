// Package scheduler provides an object capable of running functions at specific times,
// while your application is running.
//
// To use it:
//
// 1) Create a new scheduler with New().
// 2) Schedule callbacks with AddAt() or AddAfter().
//
// AddAt() and AddAfter() are thread safe - can be invoked from any thread.
//
// It is important for the callback function to return immediately: any work done
// within the callback will block following events from running.
//
// Typically, the callback will start a dedicated coroutine, or queue work to be
// run by a workpool.
//
// To prevent deadlocks, it is not safe to call AddAt() or AddAfter() from within a callback.
// Invoke those functions from a coroutine, with go AddAt() or go AddAfter() instead.
//
// You can use the helpers in the WorkPool package to collect the results or
// errors returned by scheduled callbacks, or to wrap the work in a dedicated coroutine.

package scheduler

import (
	"container/heap"
	"sync"
	"time"
)

type Work func()

type TimeSource func() time.Time

type TimeAfter func(time.Duration) <-chan time.Time

type event struct {
	when time.Time
	work Work
}

type Scheduler struct {
	// Function returning the current time.
	ts TimeSource
	// Function returning a channel to wait for timer elapsed events.
	ta TimeAfter
	// How long to wait before rechecking the heap (this is for defense in depth).
	tw time.Duration

	q  chan event
	wg *sync.WaitGroup
}

type Modifier func(*Scheduler)

type Modifiers []Modifier

func WithTimeSource(ts TimeSource) Modifier {
	return func(s *Scheduler) {
		s.ts = ts
	}
}

func WithWaitGroup(wg *sync.WaitGroup) Modifier {
	return func(s *Scheduler) {
		s.wg = wg
	}
}

func WithTimeAfter(ta TimeAfter) Modifier {
	return func(s *Scheduler) {
		s.ta = ta
	}
}

func WithTimeWait(td time.Duration) Modifier {
	return func(s *Scheduler) {
		s.tw = td
	}
}

// New creates a new scheduler.
func New(mods ...Modifier) *Scheduler {
	scheduler := &Scheduler{
		ts: time.Now,
		ta: time.After,
		tw: 1 * time.Minute,

		wg: &sync.WaitGroup{},
		q:  make(chan event),
	}
	for _, m := range mods {
		m(scheduler)
	}

	go scheduler.Loop()
	return scheduler
}

func (s *Scheduler) AddAt(t time.Time, work Work) {
	s.wg.Add(1)
	s.q <- event{work: work, when: t}
}

func (s *Scheduler) AddAfter(d time.Duration, work Work) {
	s.wg.Add(1)
	s.q <- event{work: work, when: s.ts().Add(d)}
}

func (s *Scheduler) Loop() {
	events := &eventHeap{}
	for {
		ev := (*event)(nil)
		when := s.tw
		if events.Len() > 0 {
			ev = (*events)[0]
			when = ev.when.Sub(s.ts())
		}

		select {
		case <-s.ta(when):
			if ev != nil {
				ev.work()
				heap.Pop(events)
				s.wg.Done()
			}
		case ev, ok := <-s.q:
			if !ok {
				return
			}
			heap.Push(events, &ev)
		}
	}
}
func (s *Scheduler) Wait() {
	s.wg.Wait()
}
func (s *Scheduler) Cancel() {
	close(s.q)
}
func (s *Scheduler) Done() {
	s.Wait()
	s.Cancel()
}
