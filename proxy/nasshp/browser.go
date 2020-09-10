package nasshp

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/gorilla/websocket"
	"sync"
	"sync/atomic"
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
	wc       *websocket.Conn
	err      error

	lock sync.RWMutex
	cond *sync.Cond

	readUntil, writtenUntil uint32

	pendingWack, pendingRack uint32 // obsolete!
}

func NewReplaceableBrowser(log logger.Logger) *ReplaceableBrowser {
	rb := &ReplaceableBrowser{
		log: log,
	}
	rb.cond = sync.NewCond(rb.lock.RLocker())
	return rb
}

func (gb *ReplaceableBrowser) Init(log logger.Logger) {
	gb.log = log
	gb.cond = sync.NewCond(gb.lock.RLocker())
}

func (gb *ReplaceableBrowser) GetWack() (*websocket.Conn, uint32, uint32, error) {
	gb.cond.L.Lock() // This is a read only lock, see how cond is created.
	defer gb.cond.L.Unlock()
	for gb.wc == nil && gb.err == nil {
		gb.cond.Wait()
	}

	wack := gb.pendingWack
	gb.pendingWack = 0
	return gb.wc, gb.pendingRack, wack, gb.err
}

func (gb *ReplaceableBrowser) GetRack() (*websocket.Conn, uint32, uint32, error) {
	gb.cond.L.Lock() // This is a read only lock, see how cond is created.
	defer gb.cond.L.Unlock()
	for gb.wc == nil && gb.err == nil {
		gb.cond.Wait()
	}

	rack := gb.pendingRack
	gb.pendingRack = 0
	return gb.wc, rack, gb.pendingWack, gb.err
}

func (gb *ReplaceableBrowser) Set(wc *websocket.Conn, rack, wack uint32) waiter {
	gb.lock.Lock() // This is an exclusive write lock.
	defer gb.lock.Unlock()
	if gb.wc == wc {
		return gb.notifier
	}
	if gb.wc != nil {
		gb.notifier <- fmt.Errorf("replaced browser connection")
		gb.wc.Close()
	}
	gb.wc = wc
	if wc == nil {
		gb.notifier = nil
		gb.pendingRack = 0
		gb.pendingWack = 0
		return nil
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

func (gb *ReplaceableBrowser) Close(err error) {
	gb.lock.Lock() // This is an exclusive write lock.
	defer gb.lock.Unlock()
	gb.err = err
	if gb.notifier != nil {
		gb.notifier <- &TerminatingError{error: err}
		gb.notifier = nil
	}
	if gb.wc != nil {
		gb.wc.Close()
		gb.wc = nil
	}
}

func (gb *ReplaceableBrowser) Error(wc *websocket.Conn, err error) {
	gb.lock.Lock() // This is an exclusive write lock.
	defer gb.lock.Unlock()

	// The browser has already gone, nothing to do here.
	if gb.wc == nil || gb.wc != wc {
		return
	}

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
	return wc, atomic.LoadUint32(&gb.readUntil), err
}

func (gb *ReplaceableBrowser) GetForSend() (*websocket.Conn, uint32, uint32, error) {
	wc, err := gb.Get()
	return wc, atomic.LoadUint32(&gb.writtenUntil), atomic.LoadUint32(&gb.readUntil), err
}

func (gb *ReplaceableBrowser) PushReadUntil(ru uint32) {
	atomic.StoreUint32(&gb.readUntil, ru)
}

func (gb *ReplaceableBrowser) PushWrittenUntil(wu uint32) {
	atomic.StoreUint32(&gb.writtenUntil, wu)
}
