package nasshp

import (
	"errors"
	"sync"
	"time"
)

type Waiter struct {
	l sync.Locker
	c chan bool
	e error
}

func NewWaiter(l sync.Locker) *Waiter {
	return &Waiter{
		l: l,
		c: make(chan bool, 1),
	}
}

func (w *Waiter) Fail(err error) {
	w.l.Lock()
	defer w.l.Unlock()
	w.e = err
	close(w.c)
}

func (w *Waiter) Signal() {
	select {
	case w.c <- true:
	default:
	}
}

func (w *Waiter) Wait() error {
	w.l.Unlock()
	_ = <-w.c
	w.l.Lock()
	return w.e
}

var ErrorExpired = errors.New("timer expired")

func (w *Waiter) WaitFor(d time.Duration) error {
	w.l.Unlock()
	select {
	case <-w.c:
		w.l.Lock()
		return w.e

	case <-time.After(d):
		w.l.Lock()
		return ErrorExpired
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
	return nil
}

func (b *BlockingReceiveWindow) ToFill() []byte {
	b.l.Lock()
	defer b.l.Unlock()
	data := b.w.ToFill()
	return data
}

func (b *BlockingReceiveWindow) Fill(size int) uint64 {
	b.l.Lock()
	defer b.l.Unlock()
	filled := b.w.Fill(size)
	b.ce.Signal()
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
	data := b.w.ToEmpty()
	return data
}

func (b *BlockingReceiveWindow) Empty(size int) {
	b.l.Lock()
	defer b.l.Unlock()
	b.w.Empty(size)
	b.cf.Signal()
}

func (b *BlockingReceiveWindow) Fail(err error) {
	b.ce.Fail(err)
	b.cf.Fail(err)
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
	b.l.Lock()
	defer b.l.Unlock()
	data := b.w.ToFill()
	return data
}

func (b *BlockingSendWindow) Fill(size int) uint64 {
	b.l.Lock()
	defer b.l.Unlock()
	filled := b.w.Fill(size)
	b.ce.Signal()
	return filled
}

func (b *BlockingSendWindow) Reset(wu uint32) error {
	b.l.Lock()
	defer b.l.Unlock()
	return b.w.Reset(wu)
}

func (b *BlockingSendWindow) AcknowledgeUntil(wu uint32) (uint64, error) {
	b.l.Lock()
	defer b.l.Unlock()
	val, err := b.w.AcknowledgeUntil(wu)
	b.cf.Signal()
	return val, err
}

func (b *BlockingSendWindow) WaitToFill() error {
	b.l.Lock()
	defer b.l.Unlock()
	for (b.w.Filled - b.w.acknowledged) >= b.max {
		if err := b.cf.Wait(); err != nil {
			return err
		}
	}
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
	return nil
}

func (b *BlockingSendWindow) ToEmpty() []byte {
	b.l.Lock()
	defer b.l.Unlock()
	data := b.w.ToEmpty()
	return data
}

func (b *BlockingSendWindow) Empty(size int) {
	b.l.Lock()
	defer b.l.Unlock()
	b.w.Empty(size)
}

func (b *BlockingSendWindow) Fail(err error) {
	b.ce.Fail(err)
	b.cf.Fail(err)
}
