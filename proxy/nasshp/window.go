package nasshp

import (
	"fmt"
	"log"
	"sync"
)

type SendWindow struct {
	buffer []byte

	ackstart, fillstart, emptystart int
	acknowledged, filled, emptied   uint64
}

func NewSendWindow(bsize int) *SendWindow {
	return &SendWindow{
		buffer: make([]byte, bsize),
	}
}

func (w *SendWindow) ToFill() []byte {
	if w.ackstart <= w.fillstart {
		if w.ackstart == 0 {
			return w.buffer[w.fillstart : len(w.buffer)-1]
		}
		return w.buffer[w.fillstart:]
	}
	return w.buffer[w.fillstart : w.ackstart-1]
}

func (w *SendWindow) Filled(size int) {
	w.filled += uint64(size)

	w.fillstart += size
	if w.fillstart == len(w.buffer) {
		w.fillstart = 0
	}
}

func (w *SendWindow) ToEmpty() []byte {
	if w.emptystart > w.fillstart {
		return w.buffer[w.emptystart:]
	}
	return w.buffer[w.emptystart:w.fillstart]
}

func (w *SendWindow) Empty(size int) {
	w.emptied += uint64(size)

	w.emptystart += size
	if w.emptystart == len(w.buffer) {
		w.emptystart = 0
	}
}

func (w *SendWindow) AcknowledgeUntil(trunc uint32) error {
	until := ToAbsolute(w.acknowledged, trunc)
	if until < w.acknowledged || until > w.emptied {
		return fmt.Errorf("ack is acknowledging more than allowed")
	}

	diff := w.acknowledged - until
	w.acknowledged = until

	w.ackstart += int(diff)
	if w.ackstart > len(w.buffer) {
		w.ackstart -= len(w.buffer)
	}
	return nil
}

// Examples:
// - let's say we only send the last digit, 0 to 9
// - current value is 13.
//   - We get 4. Likely means 14.
//   - We get 9. 19? or 9?
// - current value is 19.
//   - We get 8. Does it mean 18? or does it mean 28?
func ToAbsolute(reference uint64, trunc uint32) uint64 {
	base := uint32(reference & 0xffffff)
	diff := uint32(0)
	if trunc >= base {
		diff = trunc - base
	} else {
		diff = 0x1000000 - (base - trunc + 1)
	}

	topbits := reference & ^uint64(0xffffff)
	switch {
	case diff > 0x7fffff && trunc > base && topbits > 0x1000000:
		return (topbits - 0x1000000) | uint64(trunc)
	case diff <= 0x7fffff && trunc < base:
		return (topbits + 0x1000000) | uint64(trunc)
	}
	return topbits | uint64(trunc)
}

func (w *SendWindow) Reset(trunc uint32) error {
	from := ToAbsolute(w.acknowledged, trunc)
	if from < w.acknowledged || from > w.filled {
		return fmt.Errorf("request ack reset outside allowed range")
	}

	diff := w.acknowledged - from
	w.acknowledged = from
	w.ackstart += int(diff)
	if w.ackstart > len(w.buffer) {
		w.ackstart -= len(w.buffer)
	}
	w.emptystart = w.ackstart

	return nil
}

type BufferPool sync.Pool

func NewBufferPool(size int) *BufferPool {
	return (*BufferPool)(&sync.Pool{
		New: func() interface{} {
			return &buffer{
				data: make([]byte, 0, size),
			}
		},
	})
}

func (bp *BufferPool) Get() *buffer {
	sp := (*sync.Pool)(bp)
	return sp.Get().(*buffer)
}

func (bp *BufferPool) Put(b *buffer) {
	// Reset buffer.
	b.data = b.data[:0]
	// Zero other fields.
	b.emptied = 0

	sp := (*sync.Pool)(bp)
	sp.Put(b)
}

type buffer struct {
	data    []byte
	emptied int

	prev, next *buffer
}

type blist buffer

func (bl *blist) Init() {
	bl.next = (*buffer)(bl)
	bl.prev = (*buffer)(bl)
}

func (bl *blist) First() *buffer {
	return bl.next
}

func (bl *blist) Last() *buffer {
	//log.Printf("last %p", bl.prev)
	return bl.prev
}

// End return the buffer pointer used to indicate the end of the chain.
func (bl *blist) End() *buffer {
	return (*buffer)(bl)
}

// blist.next = b1
// blist.prev = b1
//
// b1.next = blist
// b1.prev = blist
//
// InsertAfter(b1, n)
//
// n.prev = b1
// n.next = b1.next = blist
// b1.next.prev = n --> blist.prev = n
// b1.next = n
//
// blist.next =b1
// b1.next = n
// n.next = blist

// blist.prev = n
// n.prev = b1
// b1.prev = blist
func (bl *blist) InsertAfter(where, what *buffer) *buffer {
	what.prev = where
	what.next = where.next

	where.next.prev = what
	where.next = what
	return what
}

func (bl *blist) Append(toadd *buffer) *buffer {
	res := bl.InsertAfter(bl.Last(), toadd)
	//log.Printf("appended %p, last %p", toadd, bl.Last())
	return res
}

func (bl *blist) Drop(todrop *buffer) *buffer {
	todrop.prev.next, todrop.next.prev = todrop.next, todrop.prev
	todrop.prev, todrop.next = nil, nil // Strictly not necessary, for defense in depth.
	return todrop
}

type ReceiveWindow struct {
	filled  uint64 // Absolute counter of bytes filled.
	reset   uint64 // Reset position
	emptied uint64 // Absolute counter of bytes emptied.

	pool   *BufferPool
	buffer blist
}

func NewReceiveWindow(pool *BufferPool) *ReceiveWindow {
	rw := &ReceiveWindow{
		pool: pool,
	}
	rw.buffer.Init()
	return rw
}

func (w *ReceiveWindow) Reset(wack uint32) error {
	value := ToAbsolute(w.emptied, wack)
	if value == w.filled {
		return nil
	}
	if value > w.filled {
		return fmt.Errorf("can't leaves gaps in receive buffer - have been asked to reset past data received")
	}

	// We are moving back in time, preparing to fill the buffer with data that was already
	// consumed, and effectively needs to be skipped.
	//
	// If this data to be skipped lands in a middle of a buffer, the code in this file cannot
	// really skip it, as the lean data structures don't allow for gaps. They only allow for
	// data to be marked as consumed at the beginning of a buffer.
	//
	// The code here ensures that any new data lands at the beginning of the buffer, so we
	// can easily skip it in case.
	if len(w.buffer.Last().data) > 0 {
		w.buffer.Append(w.pool.Get())
	}

	w.reset = value
	return nil
}

func (w *ReceiveWindow) ToFill() []byte {
	last := w.buffer.Last()
	if cap(last.data) > len(last.data) {
		log.Printf("existing")
		return last.data[len(last.data):cap(last.data)]
	}

	log.Printf("new - %p %d %d", last, cap(last.data), len(last.data))
	last = w.buffer.Append(w.pool.Get())
	log.Printf("created - %p %d %d", last, cap(last.data), len(last.data))
	return last.data[:cap(last.data)]
}

func (w *ReceiveWindow) Filled(size int) {
	last := w.buffer.Last()

	// If we already emptied this data, we need to discard it.
	log.Printf("%p filled %d emptied %d reset %d", last, w.filled, w.emptied, w.reset)
	if w.reset != 0 && w.reset < w.filled {
		sentalready := w.filled - w.reset
		if sentalready > uint64(size) {
			w.reset += uint64(size)
			return
		}
		log.Printf("sentalready %d", int(sentalready))
		last.emptied = int(sentalready) // Skip the first sentalready bytes of the buffer when ToEmpty is called.
		w.reset = 0

		w.filled += uint64(size) - sentalready
	} else {
		w.filled += uint64(size)
	}

	last.data = last.data[0 : len(last.data)+size]
}

func (w *ReceiveWindow) ToEmpty() []byte {
	first := w.buffer.First()
	return first.data[first.emptied:]
}

func (w *ReceiveWindow) Empty(size int) {
	w.emptied += uint64(size)
	log.Printf("incremented emptied by %d, emp %d", size, w.emptied)
	for size > 0 {
		first := w.buffer.First()
		filled := first.emptied + size
		if filled < len(first.data) {
			first.emptied += size
			break
		}

		// If we are here, we have exhausted the buffer.
		//
		// If there are no more buffers, this is the last one, don't throw it away.
		// Rather, reset it so it gets reused. This is very important: on a typical
		// application, with small writes, where the reader catches up immediately,
		// it allows to re-use the same buffer over and over.
		if first == w.buffer.Last() {
			first.emptied = 0
			first.data = first.data[:0]
			break
		}

		size -= len(first.data) - first.emptied
		log.Printf("dropping buffer - e %d l %d", first.emptied, len(first.data))
		w.pool.Put(w.buffer.Drop(first))
	}
}
