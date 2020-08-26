package workpool

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/kflags"
	"runtime"
	"sync"
)

type Work func()

type options struct {
	workers int

	normalQueueSize    int
	immediateQueueSize int
	wg                 *sync.WaitGroup
}

func DefaultOptions() *options {
	return &options{
		workers: runtime.NumCPU(),
		wg:      &sync.WaitGroup{},
	}
}

type WorkPool struct {
	// nq: Normal Queue, iq: Immediage Queue.
	// The only difference is that the iq is used more rarely, and likely to be empty.
	nq, iq chan Work
	wg     *sync.WaitGroup
}

type Modifier func(*options) error

type Modifiers []Modifier

type Flags struct {
	QueueSize          int
	ImmediateQueueSize int
	Workers            int
}

func DefaultFlags() *Flags {
	options := DefaultOptions()
	return &Flags{
		Workers: options.workers,
	}
}

func (cf *Flags) Register(set kflags.FlagSet, prefix string) *Flags {
	set.IntVar(&cf.QueueSize, prefix+"workpool-queue-size", cf.QueueSize, "How many jobs to queue before blocking the thread adding them")
	set.IntVar(&cf.ImmediateQueueSize, prefix+"workpool-immediate-queue-size", cf.ImmediateQueueSize, "How many immediate jobs to allow in queue before blocking the thread adding them")
	set.IntVar(&cf.Workers, prefix+"workpool-workers", cf.Workers, "How many workers to run in parallel to perform the jobs")
	return cf
}

func FromFlags(flags *Flags) Modifier {
	return func(o *options) error {
		if flags == nil {
			return nil
		}

		if flags.QueueSize < 0 {
			return kflags.NewUsageError(fmt.Errorf("invalid workpool-queue-size %d - must be > 0", flags.QueueSize))
		}
		if flags.ImmediateQueueSize < 0 {
			return kflags.NewUsageError(fmt.Errorf("invalid workpool-immediate-queue-size %d - must be > 0", flags.ImmediateQueueSize))
		}
		if flags.Workers < 0 {
			return kflags.NewUsageError(fmt.Errorf("invalid workpool-workers %d - must be > 0", flags.Workers))
		}

		o.normalQueueSize = flags.QueueSize
		o.immediateQueueSize = flags.ImmediateQueueSize
		o.workers = flags.Workers
		return nil
	}
}

func WithQueueSize(size int) Modifier {
	return func(o *options) error {
		o.normalQueueSize = size
		return nil
	}
}
func WithImmediateQueueSize(size int) Modifier {
	return func(o *options) error {
		o.immediateQueueSize = size
		return nil
	}
}
func WithWorkers(size int) Modifier {
	return func(o *options) error {
		o.workers = size
		return nil
	}
}
func WithWaitGroup(wg *sync.WaitGroup) Modifier {
	return func(o *options) error {
		o.wg = wg
		return nil
	}
}

// Creates a new WorkPool.
func New(mods ...Modifier) (*WorkPool, error) {
	o := DefaultOptions()
	for _, m := range mods {
		if err := m(o); err != nil {
			return nil, err
		}
	}

	wp := &WorkPool{
		nq: make(chan Work, o.normalQueueSize),
		iq: make(chan Work, o.immediateQueueSize),
		wg: o.wg,
	}

	for ix := 0; ix < o.workers; ix++ {
		go wp.Do()
	}

	return wp, nil
}

// Add adds work to be completed from one of the goroutines managed by the WorkPool.
func (wp *WorkPool) Add(work Work) {
	wp.wg.Add(1)
	wp.nq <- work
}

// AddImmediate is just like Add: it adds work to be completed from one of the goroutines managed by the WorkPool.
//
// The difference between Add and AddImmediate is that they use two different queues.
// Assuming that you normally use Add to queue your work, calling AddImmediate would
// bypass any work queued by Add, and have some work run as soon as a worker becomes
// available, rather than at the end of the queue.
func (wp *WorkPool) AddImmediate(work Work) {
	wp.wg.Add(1)
	wp.iq <- work
}

// Do runs an infinite loop processing all the work requested.
//
// Normally, Do() is invoked with 'go wp.Do()' from New,
// but you can call 'go wp.Do()' manually to spawn more workers.
func (wp *WorkPool) Do() {
	var work Work
	var ok bool
	for {
		// Always tries to consume from the immediate queue first.
		select {
		case work, ok = <-wp.iq:
			if !ok {
				return
			}
			work()
			wp.wg.Done()
			continue
		default:
		}

		// Now consume from whichever queue.
		select {
		case work, ok = <-wp.nq:
		case work, ok = <-wp.iq:
		}
		if !ok {
			return
		}

		work()
		wp.wg.Done()
	}
}

// Wait blocks until all the work queued in the configured WaitGroup is completed.
func (wp *WorkPool) Wait() {
	wp.wg.Wait()
}

// Cancel causes the WorkPool to stop processing any work immediately, and terminate all the workers.
// The WorkPool can no longer be used after Cancel() is called.
func (wp *WorkPool) Cancel() {
	close(wp.nq)
	close(wp.iq)
}

// Done waits for all the work queued to be completed, to then terminate all the workers.
// The WorkPool can no longer be used after Done() is called.
func (wp *WorkPool) Done() {
	wp.Wait()
	wp.Cancel()
}
