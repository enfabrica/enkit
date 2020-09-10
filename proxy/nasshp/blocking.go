package nasshp

import (
	"errors"
	"log"
	"sync"
	"time"
)

type Waiter struct {
	l sync.Locker
	c chan error
}

func NewWaiter(l sync.Locker) *Waiter {
	return &Waiter{
		l: l,
		c: make(chan error, 1),
	}
}

func (w *Waiter) Channel() chan error {
	return w.c
}

func (w *Waiter) Fail(err error) {
	w.c <- err
}

func (w *Waiter) Signal() {
	select {
	case w.c <- nil:
	default:
	}
}

func (w *Waiter) Wait() error {
	w.l.Unlock()
	defer w.l.Lock()
	return <-w.c
}

var ErrorExpired = errors.New("timer expired")

func (w *Waiter) WaitFor(d time.Duration) error {
	w.l.Unlock()
	defer w.l.Lock()
	select {
	case err := <-w.c:
		return err
	case <-time.After(d):
		return nil
	}
}

// BlockingSendWindow allows to split the filling and emptying of a SendWindow across different goroutines.
//
// Specifically, it assumes that there is one goroutine calling ToFill and Fill, and another goroutine
// calling ToEmpty, Empty, and possibly Reset and Acknowledge.
//
// Note that the BlockingSendWindow still supports at most one sender and at most one receiver, not more.
type BlockingSendWindow struct {
	w SendWindow
	l sync.Mutex

	max uint64 // Maximum amount of pending bytes.

	cf *Waiter // Wait for fill to be ready.
	ce *Waiter // Wait for empty to be ready.
}

// BlockingReceiveWindow allows to split the filling and emptying of a ReceiveWindow across different goroutines.
//
// Specifically, it assumes that there is one goroutine calling ToFill and Fill, and another goroutine
// calling ToEmpty, Empty, and Reset.
//
// Having more than one filling or more than one empting goroutine is unsupported.
type BlockingReceiveWindow struct {
	w ReceiveWindow
	l sync.Mutex

	max uint64 // Maximum amount of pending bytes.

	cf *Waiter // Wait for fill to be ready.
	ce *Waiter // Wait for empty to be ready.
}

func NewBlockingReceiveWindow(pool *BufferPool, max uint64) *BlockingReceiveWindow {
	bw := &BlockingReceiveWindow{}
	bw.w.pool = pool
	bw.w.buffer.Init()
	bw.ce = NewWaiter(&bw.l)
	bw.cf = NewWaiter(&bw.l)
	bw.max = max
	return bw
}

func (b *BlockingReceiveWindow) WaitToFill() error {
	b.l.Lock()
	defer b.l.Unlock()
	for (b.w.Filled - b.w.Emptied) >= b.max {
		if err := b.cf.Wait(); err != nil {
			return err
		}
	}
	return nil
}

func (b *BlockingReceiveWindow) WaitToEmpty() error {
	b.l.Lock()
	defer b.l.Unlock()
	for len(b.w.ToEmpty()) == 0 {
		if err := b.ce.Wait(); err != nil {
			return err
		}
	}
	log.Printf("receive to empty %d", len(b.w.ToEmpty()))
	return nil
}

func (b *BlockingReceiveWindow) ToFill() []byte {
	// FIXME: Enforce a limit, block having too much data in memory.
	b.l.Lock()
	defer b.l.Unlock()
	return b.w.ToFill()
}

func (b *BlockingReceiveWindow) Fill(size int) uint64 {
	b.l.Lock()
	defer b.l.Unlock()
	filled := b.w.Fill(size)
	b.ce.Signal()
	log.Printf("receive filled %d (%d)", filled, size)
	return filled
}

func (b *BlockingReceiveWindow) Reset(wu uint32) error {
	b.l.Lock()
	defer b.l.Unlock()
	return b.w.Reset(wu)
}

func (b *BlockingReceiveWindow) ToEmpty() []byte {
	b.l.Lock()
	defer b.l.Unlock()
	return b.w.ToEmpty()
}

func (b *BlockingReceiveWindow) Empty(size int) {
	b.l.Lock()
	defer b.l.Unlock()
	b.w.Empty(size)
	b.cf.Signal()
}

func NewBlockingSendWindow(pool *BufferPool, max uint64) *BlockingSendWindow {
	bw := &BlockingSendWindow{}
	bw.w.pool = pool
	bw.w.buffer.Init()
	bw.w.pending.Init()
	bw.ce = NewWaiter(&bw.l)
	bw.cf = NewWaiter(&bw.l)
	bw.max = max
	return bw
}

func (b *BlockingSendWindow) ToFill() []byte {
	// FIXME: Enforce a limit, block having too much data in memory.
	b.l.Lock()
	defer b.l.Unlock()
	return b.w.ToFill()
}

func (b *BlockingSendWindow) Fill(size int) uint64 {
	b.l.Lock()
	defer b.l.Unlock()
	filled := b.w.Fill(size)
	b.ce.Signal()
	log.Printf("send filled %d (%d)", filled, size)
	return filled
}

func (b *BlockingSendWindow) Reset(wu uint32) error {
	b.l.Lock()
	defer b.l.Unlock()
	return b.w.Reset(wu)
}

func (b *BlockingSendWindow) AcknowledgeUntil(wu uint32) error {
	b.l.Lock()
	defer b.l.Unlock()
	err := b.w.AcknowledgeUntil(wu)
	b.cf.Signal()
	return err
}

func (b *BlockingSendWindow) WaitToFill() error {
	b.l.Lock()
	defer b.l.Unlock()
	for (b.w.Filled - b.w.acknowledged) >= b.max {
		if err := b.cf.Wait(); err != nil {
			return err
		}
	}
	log.Printf("send to empty %d", len(b.w.ToEmpty()))
	return nil
}

func (b *BlockingSendWindow) WaitToEmpty(d time.Duration) error {
	b.l.Lock()
	defer b.l.Unlock()
	for len(b.w.ToEmpty()) == 0 {
		if err := b.ce.WaitFor(d); err != nil {
			return err
		}
	}
	log.Printf("send to empty %d", len(b.w.ToEmpty()))
	return nil
}

func (b *BlockingSendWindow) ToEmpty() []byte {
	b.l.Lock()
	defer b.l.Unlock()
	return b.w.ToEmpty()
}

func (b *BlockingSendWindow) Empty(size int) {
	b.l.Lock()
	defer b.l.Unlock()
	b.w.Empty(size)
}
