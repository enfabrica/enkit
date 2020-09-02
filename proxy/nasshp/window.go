package nasshp

import (
	"fmt"
	"sync"
)

type buffer struct {
	// We use the slice to:
	// - maintain a pointer to the start of the data.
	// - know the amount of memory allocated - cap(data).
	// - know the amount of memory actually used - len(data).
	data []byte
	// Offset indicates how many bytes at the beginning of data to skip.
	// This way, we don't have to update the data pointer.
	offset int
	// Acknowledged indicates how many bytes have been acknowledged.
	acknowledged int

	// Used to maintain a chain of buffers.
	prev, next *buffer
}

type blist buffer

func (bl *blist) Init() *blist {
	bl.next = (*buffer)(bl)
	bl.prev = (*buffer)(bl)
	return bl
}

func (bl *blist) First() *buffer {
	return bl.next
}

func (bl *blist) Last() *buffer {
	return bl.prev
}

func (bl *blist) InsertListBefore(where *buffer, list *blist) {
	first := list.First()
	where.prev.next = first
	first.prev = where.prev

	last := list.Last()
	where.prev = last
	last.next = where

	// Make sure there are no lingering pointers in the original list.
	list.Init()
}

func (bl *blist) InsertAfter(where, what *buffer) *buffer {
	what.prev = where
	what.next = where.next

	where.next.prev = what
	where.next = what
	return what
}

func (bl *blist) Append(toadd *buffer) *buffer {
	res := bl.InsertAfter(bl.Last(), toadd)
	return res
}

func (bl *blist) Drop(todrop *buffer) *buffer {
	todrop.prev.next, todrop.next.prev = todrop.next, todrop.prev
	return todrop
}

func (bl *blist) End() *buffer {
	return (*buffer)(bl)
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
	// Ensure that when the buffer is recycled, it is empty.
	b.data = b.data[:0]
	b.offset = 0
	b.acknowledged = 0
	// This is pretty much only for defense in depth.
	b.prev, b.next = nil, nil

	sp := (*sync.Pool)(bp)
	sp.Put(b)
}

type SendWindow struct {
	filled       uint64 // Absolute counter of bytes filled.
	acknowledged uint64 // Absolute counter of bytes acknowledged from this window.
	emptied      uint64 // Absolute counter of bytes consumed from this window.

	pool    *BufferPool
	buffer  blist
	pending blist
}

func NewSendWindow(pool *BufferPool) *SendWindow {
	w := &SendWindow{pool: pool}

	w.buffer.Init()
	w.pending.Init()
	return w
}

func (w *SendWindow) ToFill() []byte {
	last := w.buffer.Last()
	if cap(last.data) > len(last.data) {
		return last.data[len(last.data):cap(last.data)]
	}

	last = w.buffer.Append(w.pool.Get())
	return last.data[:cap(last.data)]
}

func (w *SendWindow) Filled(size int) {
	last := w.buffer.Last()

	w.filled += uint64(size)
	last.data = last.data[0 : len(last.data)+size]
}

func (w *SendWindow) ToEmpty() []byte {
	first := w.buffer.First()
	return first.data[first.offset:]
}

func (w *SendWindow) Empty(size int) {
	w.emptied += uint64(size)
	for size > 0 {
		first := w.buffer.First()
		filled := first.offset + size
		if filled < len(first.data) {
			first.offset += size
			break
		}

		size -= len(first.data) - first.offset

		// Offset = 0 is important if the ack # is reset, and we have to go back in time.
		first.offset = 0
		w.pending.Append(w.buffer.Drop(first))
	}
}

func (w *SendWindow) Acknowledge(size int) {
	w.acknowledged += uint64(size)

	// For anything in pending, we can ignore offset, as the buffer has already been processed.
	for {
		if size <= 0 {
			return
		}

		first := w.pending.First()
		if first == w.pending.End() {
			break
		}

		acked := first.acknowledged + size
		if acked < len(first.data) {
			first.acknowledged += size
			return
		}

		size -= len(first.data) - first.acknowledged
		w.pool.Put(w.pending.Drop(first))
	}

	// If we are here, we reached the end of pending buffers, and still have data to acknowledge.

	first := w.buffer.First()
	if first.acknowledged+size < first.offset {
		first.acknowledged += size
		return
	}

	first.acknowledged = first.offset
}

func (w *SendWindow) AcknowledgeUntil(trunc uint32) error {
	value := ToAbsolute(w.acknowledged, trunc)
	if value > w.emptied {
		return fmt.Errorf("impossible ack: acknowledges data that was never sent")
	}
	if value < w.acknowledged {
		return fmt.Errorf("political ack: the data had been acked already, but this ack claims it never was - and now wants to resume from data we no longer have")
	}
	w.Acknowledge(int(value - w.acknowledged))
	return nil
}

func (w *SendWindow) Reset(trunc uint32) error {
	value := ToAbsolute(w.acknowledged, trunc)
	if value > w.emptied {
		return fmt.Errorf("invalid ack, puts us past send buffer")
	}
	if value < w.acknowledged {
		return fmt.Errorf("invalid ack, requires going before last ack, for which we have no buffer")
	}

	w.Acknowledge(int(value - w.acknowledged))
	w.buffer.InsertListBefore(w.buffer.First(), &w.pending)

	w.buffer.First().offset = w.buffer.First().acknowledged
	w.emptied = w.acknowledged
	w.filled = w.acknowledged
	return nil
}

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
		return last.data[len(last.data):cap(last.data)]
	}

	last = w.buffer.Append(w.pool.Get())
	return last.data[:cap(last.data)]
}

func (w *ReceiveWindow) Filled(size int) {
	last := w.buffer.Last()

	// If we already emptied this data, we need to discard it.
	if w.reset != 0 && w.reset < w.filled {
		sentalready := w.filled - w.reset
		if sentalready > uint64(size) {
			w.reset += uint64(size)
			return
		}
		last.offset = int(sentalready) // Skip the first sentalready bytes of the buffer when ToEmpty is called.
		w.reset = 0

		w.filled += uint64(size) - sentalready
	} else {
		w.filled += uint64(size)
	}

	last.data = last.data[0 : len(last.data)+size]
}

func (w *ReceiveWindow) ToEmpty() []byte {
	first := w.buffer.First()
	return first.data[first.offset:]
}

func (w *ReceiveWindow) Empty(size int) {
	w.emptied += uint64(size)
	for size > 0 {
		first := w.buffer.First()
		filled := first.offset + size
		if filled < len(first.data) {
			first.offset += size
			break
		}

		// If we are here, we have exhausted the buffer.
		//
		// If there are no more buffers, this is the last one, don't throw it away.
		// Rather, reset it so it gets reused. This is very important: on a typical
		// application, with small writes, where the reader catches up immediately,
		// it allows to re-use the same buffer over and over.
		if first == w.buffer.Last() {
			first.offset = 0
			first.data = first.data[:0]
			break
		}

		size -= len(first.data) - first.offset
		w.pool.Put(w.buffer.Drop(first))
	}
}
