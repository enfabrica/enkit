package nasshp

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/proxy/utils"
	"github.com/gorilla/websocket"
	"sync"
	"sync/atomic"
	"time"
)

type waiter chan error

func (w waiter) Wait() error {
	return <-w
}

func (w waiter) Channel() chan error {
	return w
}

type ReplaceableBrowser struct {
	log logger.Logger

	notifier chan error
	counters *BrowserWindowCounters

	started utils.AtomicTime // epoch when connection was started.
	paused  utils.AtomicTime // epoch when connection was last paused.

	wc   *websocket.Conn // Protected by lock.
	err  error           // Protected by lock.
	lock sync.RWMutex
	cond *sync.Cond

	readUntil, writtenUntil uint32

	// Contended, use atomic.Load/Store to read/write.
	pendingWack, pendingRack uint32
}

func NewReplaceableBrowser(log logger.Logger, counters *BrowserWindowCounters) *ReplaceableBrowser {
	if counters == nil {
		counters = &BrowserWindowCounters{}
	}

	rb := &ReplaceableBrowser{
		log:      log,
		counters: counters,
	}
	rb.cond = sync.NewCond(rb.lock.RLocker())
	return rb
}

func (gb *ReplaceableBrowser) Init(log logger.Logger, counters *BrowserWindowCounters) {
	gb.log = log
	gb.counters = counters
	gb.cond = sync.NewCond(gb.lock.RLocker())
}

func (gb *ReplaceableBrowser) GetWack() (*websocket.Conn, uint32, uint32, error) {
	gb.cond.L.Lock() // This is a read only lock, see how cond is created.
	defer gb.cond.L.Unlock()
	for gb.wc == nil && gb.err == nil {
		gb.cond.Wait()
	}

	wack := atomic.SwapUint32(&gb.pendingWack, 0)
	return gb.wc, atomic.LoadUint32(&gb.pendingRack), wack, gb.err
}

func (gb *ReplaceableBrowser) GetRack() (*websocket.Conn, uint32, uint32, error) {
	gb.cond.L.Lock() // This is a read only lock, see how cond is created.
	defer gb.cond.L.Unlock()
	for gb.wc == nil && gb.err == nil {
		gb.cond.Wait()
	}

	rack := atomic.SwapUint32(&gb.pendingRack, 0)
	return gb.wc, rack, atomic.LoadUint32(&gb.pendingWack), gb.err
}

func (gb *ReplaceableBrowser) Set(wc *websocket.Conn, rack, wack uint32) waiter {
	gb.lock.Lock() // This is an exclusive write lock.
	defer gb.lock.Unlock()
	if gb.wc == wc {
		return gb.notifier
	}
	if gb.wc != nil {
		gb.counters.BrowserWindowReplaced.Increment()

		gb.notifier <- fmt.Errorf("replaced browser connection")
		gb.wc.Close()
	}
	gb.wc = wc
	if wc == nil {
		gb.counters.BrowserWindowReset.Increment()

		gb.notifier = nil
		gb.pendingRack = 0
		gb.pendingWack = 0
		return nil
	}

	if rack == 0 && wack == 0 {
		gb.counters.BrowserWindowStarted.Increment()
		gb.started.Set(time.Now())
	} else {
		gb.counters.BrowserWindowResumed.Increment()
		gb.paused.Reset()
	}

	gb.pendingRack = rack
	gb.pendingWack = wack
	gb.notifier = make(chan error, 1)
	gb.cond.Broadcast()
	return gb.notifier
}

type TerminatingError struct {
	error
}

func (te *TerminatingError) Unwrap() error {
	return te.error
}

func (gb *ReplaceableBrowser) Close(err error) {
	gb.lock.Lock() // This is an exclusive write lock.
	defer gb.lock.Unlock()
	gb.err = err
	if gb.notifier != nil {
		gb.counters.BrowserWindowStopped.Increment()

		gb.notifier <- &TerminatingError{error: err}
		gb.notifier = nil
	}
	if gb.wc != nil {
		gb.counters.BrowserWindowClosed.Increment()

		gb.wc.Close()
		gb.wc = nil
	}
	gb.cond.Broadcast()
}

func (gb *ReplaceableBrowser) Error(wc *websocket.Conn, err error) {
	gb.lock.Lock() // This is an exclusive write lock.
	defer gb.lock.Unlock()

	// The browser has already gone, nothing to do here.
	if gb.wc == nil || gb.wc != wc {
		return
	}

	gb.paused.Set(time.Now())
	gb.counters.BrowserWindowOrphaned.Increment()

	gb.notifier <- err
	gb.wc.Close()
	gb.wc = nil
	gb.notifier = nil
}

func (gb *ReplaceableBrowser) Get() (*websocket.Conn, error) {
	gb.cond.L.Lock() // This is a read only lock, see how cond is created.
	defer gb.cond.L.Unlock()
	for gb.wc == nil && gb.err == nil {
		gb.cond.Wait()
	}
	return gb.wc, gb.err
}

func (gb *ReplaceableBrowser) GetWriteReadUntil() (uint32, uint32) {
	return atomic.LoadUint32(&gb.writtenUntil), atomic.LoadUint32(&gb.readUntil)
}

func (gb *ReplaceableBrowser) GetForReceive() (*websocket.Conn, uint32, error) {
	wc, err := gb.Get()
	ru := atomic.LoadUint32(&gb.readUntil)
	return wc, ru, err
}

func (gb *ReplaceableBrowser) GetForSend() (*websocket.Conn, uint32, uint32, error) {
	wc, err := gb.Get()
	wu := atomic.LoadUint32(&gb.writtenUntil)
	ru := atomic.LoadUint32(&gb.readUntil)
	return wc, wu, ru, err
}

func (gb *ReplaceableBrowser) PushReadUntil(ru uint32) {
	atomic.StoreUint32(&gb.readUntil, ru)
}

func (gb *ReplaceableBrowser) PushWrittenUntil(wu uint32) {
	atomic.StoreUint32(&gb.writtenUntil, wu)
}
