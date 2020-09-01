package nasshp

import (
	"github.com/stretchr/testify/assert"
	"log"
	"testing"
)

var quote = "There are no facts, only interpretations." // 41 bytes long.

func TestToAbsolute(t *testing.T) {
	log.Printf("simple batch")
	for i := 0; i < 12; i++ {
		assert.Equal(t, uint64(i+0x7ffffa), ToAbsolute(uint64(0), uint32(i+0x7ffffa)))
	}

	// Positive delta: the last 24 bits exceed the current 24 bits.
	// Numbers have been picked so in 6 iterations the delta exceeds 2**23
	log.Printf("positive delta")
	for i := 0; i < 12; i++ {
		if i < 6 {
			assert.Equal(t, uint64(i+0x107ffffa), ToAbsolute(uint64(0x10000000), uint32(i+0x7ffffa)))
		} else {
			assert.Equal(t, uint64(i+0xf7ffffa), ToAbsolute(uint64(0x10000000), uint32(i+0x7ffffa)))
		}
	}

	// Negative delta: the last 24 bits lare less than the current 24 bits.
	log.Printf("negative delta")
	for i := 0; i < 12; i++ {
		if i <= 6 {
			assert.Equal(t, uint64(0x10000000+12-i), ToAbsolute(uint64(0x107fffff+6), uint32(12-i)))
		} else {
			assert.Equal(t, uint64(0x11000000+12-i), ToAbsolute(uint64(0x107fffff+6), uint32(12-i)))
		}
	}
}

func TestReceiveWindowSimple(t *testing.T) {
	bsize := 8192
	p := NewBufferPool(bsize)
	w := NewReceiveWindow(p)

	data := w.ToFill()
	assert.Equal(t, bsize, len(data))
	copy(data, quote)
	w.Filled(len(quote))

	data = w.ToFill()
	assert.Equal(t, bsize-len(quote), len(data))
	copy(data, quote)
	w.Filled(len(quote))

	data = w.ToFill()
	assert.Equal(t, bsize-2*len(quote), len(data))

	data = w.ToEmpty()
	assert.Equal(t, 2*len(quote), len(data))
	data = w.ToEmpty()
	assert.Equal(t, 2*len(quote), len(data))
	w.Empty(1)
	data = w.ToEmpty()
	assert.Equal(t, 2*len(quote)-1, len(data))
}

func TestReceiveWindowLoop(t *testing.T) {
	bsize := 8192
	p := NewBufferPool(bsize)
	w := NewReceiveWindow(p)

	assert.Equal(t, len(w.buffer.data), 0)
	assert.Equal(t, cap(w.buffer.data), 0)

	for i := 0; i < 10000; i++ {
		data := w.ToFill()
		l := copy(data, quote)
		w.Filled(l)
	}

	j := 0
	for ; ; j++ {
		d := w.ToEmpty()
		if len(d) <= 0 {
			break
		}
		w.Empty(len(d))
	}
	assert.Equal(t, j, int(len(quote)*10000)/bsize)
}

func TestReceiveWindowReset(t *testing.T) {
	bsize := 8192
	p := NewBufferPool(bsize)
	w := NewReceiveWindow(p)

	// Fill 41 bytes of data (length of the quote)
	data := w.ToFill()
	l := copy(data, quote)
	assert.Equal(t, len(quote), l)
	w.Filled(l)

	e := w.ToEmpty()
	assert.Equal(t, len(quote), len(e))

	// Should fail, resetting to 53 creates a 12 btyes gap in the buffer.
	err := w.Reset(53)
	assert.NotNil(t, err)
	// Should also fail, 1 byte gap.
	err = w.Reset(42)
	assert.NotNil(t, err)
	// Success, practically a noop.
	err = w.Reset(41)
	assert.Nil(t, err)

	// No change in the data we can write out.
	e = w.ToEmpty()
	assert.Equal(t, len(quote), len(e))

	// Now we really moved back the needle.
	err = w.Reset(33)
	assert.Nil(t, err)

	// Still, we have the same data to write out, as no more data arrived.
	e = w.ToEmpty()
	assert.Equal(t, len(quote), len(e))

	// Let's fill in some data (this would be skipped)
	w.Filled(5)
	e = w.ToEmpty()
	assert.Equal(t, len(quote), len(e))

	// Part of this write should be skipped.
	data = w.ToFill()
	copy(data, quote[0:5])
	w.Filled(5)

	// Let's consume the buffer.
	log.Printf("consuming")
	data = w.ToEmpty()
	assert.Equal(t, quote, string(data))
	w.Empty(len(data))
	log.Printf("consumed")

	data = w.ToEmpty()
	assert.Equal(t, quote[3:5], string(data))
	w.Empty(len(data))

	data = w.ToFill()
	log.Printf("filling more %d", len(data))
	copy(data, quote)
	w.Filled(len(quote))

	data = w.ToEmpty()
	assert.Equal(t, quote, string(data))
	w.Empty(len(data))
}
