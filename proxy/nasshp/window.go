package nasshp

import (
	"fmt"
	"sync"
)

// blist implements a bi-directional linked list of buffer nodes.
//
// The list is conveniently represented as a node, where next points
// to the first element of the list, and prev points to the last.
//
// The end of the list is not represented by nil, but by a pointer
// to the sentinel node. The sentinel node happens to be the list
// itself.
//
// data is left empty. When this is used with buffers of fixed length,
// a 0-byte data can also be used to identify the end of the list, which
// is very convenient as code can be written to just check how much data
// is left in the next buffer, without ever worrying if it's the end
// node or not.
type blist buffer

// buffer represents a node in a bidirectional linked list and actually
// holds a pointer to an array of bytes containing the data.
//
// data is store in the array contiguously, starting from offset up to
// the len() of the data array. The size of the buffer can be determine
// using cap() of data.
type buffer struct {
	// We use the slice to:
	// - maintain a pointer to the start of the data.
	// - know the amount of memory allocated - cap(data).
	// - know the amount of memory actually used - len(data).
	data []byte
	// Offset indicates how many bytes at the beginning of data to skip.
	// This way, we don't have to update the data pointer, which we have
	// no way to recover from the data slice.
	offset int
	// Acknowledged indicates how many bytes have been acknowledged.
	acknowledged int

	// Used to maintain a chain of buffers.
	prev, next *buffer
}

// Iterate invokes the supplied function for each buffer in the list.
func (bl *blist) Iterate(callback func(*buffer)) {
	for cursor := bl.First(); cursor != nil && cursor != bl.End(); cursor = cursor.next {
		callback(cursor)
	}
}

// Init initializes a bidirectional linked list in place.
//
// If not invoked on a blist, your code will crash in horrible horrible ways.
// Won't make your buffers great again.
func (bl *blist) Init() *blist {
	bl.next = (*buffer)(bl)
	bl.prev = (*buffer)(bl)
	return bl
}

// NewBList returns a newly initalized blist.
func NewBList() *blist {
	b := &blist{}
	b.Init()
	return b
}

// First returns the first node in the list.
func (bl *blist) First() *buffer {
	return bl.next
}

// Last returns the last node in the list.
func (bl *blist) Last() *buffer {
	return bl.prev
}

// InsertListBefore prepends a whole list before the specified node.
//
// The old list is left empty, ready to be used again.
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

// InsertAfter inserts the specified node after the one specified.
func (bl *blist) InsertAfter(where, what *buffer) *buffer {
	what.prev = where
	what.next = where.next

	where.next.prev = what
	where.next = what
	return what
}

// Append adds a node at the end of a linked list.
func (bl *blist) Append(toadd *buffer) *buffer {
	res := bl.InsertAfter(bl.Last(), toadd)
	return res
}

// Drop removes a node from the linked list.
func (bl *blist) Drop(todrop *buffer) *buffer {
	todrop.prev.next, todrop.next.prev = todrop.next, todrop.prev
	return todrop
}

// End returns the sentinel node used to identify the end of the list.
func (bl *blist) End() *buffer {
	return (*buffer)(bl)
}

// BufferPool is a sync.Pool of buffers, used to allocate (and free)
// nodes used by the window implementation below.
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
	Filled  uint64 // Absolute counter of bytes Filled.
	Emptied uint64 // Absolute counter of bytes consumed from this window.

	acknowledged uint64 // Absolute counter of bytes acknowledged from this window.

	pool    *BufferPool
	buffer  blist
	pending blist
}

func (sw *SendWindow) Drop() {
	sw.buffer.Iterate(func(buffer *buffer) {
		sw.pool.Put(buffer)
	})
	sw.pending.Iterate(func(buffer *buffer) {
		sw.pool.Put(buffer)
	})
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

func (w *SendWindow) Fill(size int) uint64 {
	last := w.buffer.Last()

	w.Filled += uint64(size)
	last.data = last.data[0 : len(last.data)+size]
	return w.Filled
}

func (w *SendWindow) ToEmpty() []byte {
	first := w.buffer.First()
	data := first.data[first.offset:]
	if len(data) == 0 && first.next != w.buffer.End() {
		first.offset = 0
		w.pending.Append(w.buffer.Drop(first))
		first = w.buffer.First()
		data = first.data[first.offset:]
	}
	return data
}

func (w *SendWindow) Empty(size int) {
	w.Emptied += uint64(size)
	for {
		first := w.buffer.First()
		occupancy := len(first.data) - first.offset

		if size < occupancy {
			first.offset += size
			break
		}

		if first == w.buffer.Last() {
			first.offset += occupancy
			break
		}

		// Offset = 0 is important if the ack # is reset, and we have to go back in time.
		first.offset = 0
		w.pending.Append(w.buffer.Drop(first))

		size -= occupancy
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

		occupancy := len(first.data) - first.acknowledged
		if size < occupancy {
			first.acknowledged += size
			return
		}

		size -= occupancy
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

func (w *SendWindow) AcknowledgeUntil(trunc uint32) (uint64, error) {
	value := ToAbsolute(w.acknowledged, trunc)
	if value > w.Emptied {
		return w.acknowledged, fmt.Errorf("impossible ack: acknowledges data that was never sent (ack: %d, emptied: %d)", value, w.Emptied)
	}
	if value < w.acknowledged {
		return w.acknowledged, fmt.Errorf("political ack: the data had been acked already (and freed), but this ack claims it never was (ack: %d, acknowledged: %d)", value, w.acknowledged)
	}
	w.Acknowledge(int(value - w.acknowledged))
	return w.acknowledged, nil
}

func (w *SendWindow) Reset(trunc uint32) error {
	value := ToAbsolute(w.acknowledged, trunc)
	if value > w.Emptied {
		return fmt.Errorf("invalid ack, puts us past send buffer (ack: %d, emptied: %d)", value, w.Emptied)
	}
	if value < w.acknowledged {
		return fmt.Errorf("invalid ack, requires going before last ack, for which we have no buffer (ack: %d, acked: %d)", value, w.acknowledged)
	}
	w.Acknowledge(int(value - w.acknowledged))

	w.buffer.First().offset = 0
	w.buffer.InsertListBefore(w.buffer.First(), &w.pending)
	w.buffer.First().offset = w.buffer.First().acknowledged

	w.Emptied = w.acknowledged
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
	Filled  uint64 // Absolute counter of bytes Filled.
	reset   uint64 // Reset position
	Emptied uint64 // Absolute counter of bytes Emptied.

	pool   *BufferPool
	buffer blist
}

func (sw *ReceiveWindow) Drop() {
	sw.buffer.Iterate(func(buffer *buffer) {
		sw.pool.Put(buffer)
	})
}

func NewReceiveWindow(pool *BufferPool) *ReceiveWindow {
	rw := &ReceiveWindow{
		pool: pool,
	}
	rw.buffer.Init()
	return rw
}

func (w *ReceiveWindow) Reset(wack uint32) error {
	value := ToAbsolute(w.Emptied, wack)
	if value == w.Filled {
		return nil
	}
	if value > w.Filled {
		return fmt.Errorf("can't leave gaps in receive buffer - have been asked to reset past data received (ack: %d, Filled: %d)", value, w.Filled)
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

func (w *ReceiveWindow) Fill(size int) uint64 {
	last := w.buffer.Last()

	// If we already emptied this data, we need to discard it.
	if w.reset != 0 && w.reset < w.Filled {
		sentalready := w.Filled - w.reset
		if sentalready > uint64(size) {
			w.reset += uint64(size)
			return w.reset
		}
		last.offset = int(sentalready) // Skip the first sentalready bytes of the buffer when ToEmpty is called.
		w.reset = 0

		w.Filled += uint64(size) - sentalready
	} else {
		w.Filled += uint64(size)
	}

	last.data = last.data[0 : len(last.data)+size]
	return w.Filled
}

func (w *ReceiveWindow) ToEmpty() []byte {
	first := w.buffer.First()
	if w.reset != 0  && first.next == w.buffer.End() {
		return nil
	}

	data := first.data[first.offset:]
	if len(data) == 0 && first.next != w.buffer.End() {
		w.pool.Put(w.buffer.Drop(first))
		first = w.buffer.First()
		data = first.data[first.offset:]
	}
	return data
}

func (w *ReceiveWindow) Empty(size int) {
	w.Emptied += uint64(size)
	for {
		first := w.buffer.First()
		occupancy := len(first.data) - first.offset

		if size < occupancy {
			first.offset += size
			break
		}

		if first == w.buffer.Last() {
			first.offset += occupancy
			break
		}

		w.pool.Put(w.buffer.Drop(first))
		size -= occupancy
	}
}
