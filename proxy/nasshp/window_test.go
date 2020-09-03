package nasshp

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

// This quote is 41 bytes long. Very convenient, as 41 is prime, and won't align with powers of 2.
var quote = "There are no facts, only interpretations."

func TestSendWindowSimpleAcknowledge(t *testing.T) {
	bsize := 32 // Small buffer size to trigger as many buffer transition as possible.
	p := NewBufferPool(bsize)
	w := NewSendWindow(p)

	err := w.AcknowledgeUntil(0)
	assert.Nil(t, err)

	err = w.AcknowledgeUntil(2)
	assert.NotNil(t, err)

	d := w.ToEmpty()
	assert.Equal(t, 0, len(d))

	d = w.ToFill()
	l := copy(d, quote)
	assert.Equal(t, bsize, l)
	w.Filled(l)

	// Still failing, as the data has not been sent.
	err = w.AcknowledgeUntil(2)
	assert.NotNil(t, err)

	// Mark 4 bytes as sent.
	d = w.ToEmpty()
	assert.Equal(t, bsize, len(d))
	w.Empty(4)
	d = w.ToEmpty()
	assert.Equal(t, bsize-4, len(d))

	// Success!
	err = w.AcknowledgeUntil(2)
	assert.Nil(t, err)

	// Shouldn't change how much data we have left to empty.
	d = w.ToEmpty()
	assert.Equal(t, bsize-4, len(d))

	// No change if we re-acknowledge the same data.
	err = w.AcknowledgeUntil(2)
	assert.Nil(t, err)
	// Error if we go back and un-acknowledge some data.
	err = w.AcknowledgeUntil(1)
	assert.NotNil(t, err)

	// Same amount of data to empty.
	d = w.ToEmpty()
	assert.Equal(t, bsize-4, len(d))

	// Acknowledge up to all the data we have.
	err = w.AcknowledgeUntil(4)
	assert.Nil(t, err)
	d = w.ToEmpty()
	assert.Equal(t, bsize-4, len(d))

	// Finally, acknowledge everything.
	w.Empty(len(d))
	err = w.AcknowledgeUntil(uint32(bsize))
	assert.Nil(t, err)

	d = w.ToEmpty()
	assert.Equal(t, 0, len(d))
	d = w.ToFill()
	assert.Equal(t, bsize, len(d))
}

func TestToAbsolute(t *testing.T) {
	t.Logf("simple batch")
	for i := 0; i < 12; i++ {
		assert.Equal(t, uint64(i+0x7ffffa), ToAbsolute(uint64(0), uint32(i+0x7ffffa)))
	}

	// Positive delta: the last 24 bits exceed the current 24 bits.
	// Numbers have been picked so in 6 iterations the delta exceeds 2**23
	t.Logf("positive delta")
	for i := 0; i < 12; i++ {
		if i < 6 {
			assert.Equal(t, uint64(i+0x107ffffa), ToAbsolute(uint64(0x10000000), uint32(i+0x7ffffa)))
		} else {
			assert.Equal(t, uint64(i+0xf7ffffa), ToAbsolute(uint64(0x10000000), uint32(i+0x7ffffa)))
		}
	}

	// Negative delta: the last 24 bits lare less than the current 24 bits.
	t.Logf("negative delta")
	for i := 0; i < 12; i++ {
		if i <= 6 {
			assert.Equal(t, uint64(0x10000000+12-i), ToAbsolute(uint64(0x107fffff+6), uint32(12-i)))
		} else {
			assert.Equal(t, uint64(0x11000000+12-i), ToAbsolute(uint64(0x107fffff+6), uint32(12-i)))
		}
	}
}

func TestSendWindowSimple(t *testing.T) {
	qb := []byte{}
	for i := 0; i < 1000; i++ {
		qb = append(qb, []byte(quote)...)
	}

	bsize := 32 // Small buffer size to trigger as many buffer transition as possible.
	p := NewBufferPool(bsize)
	w := NewSendWindow(p)

	// Fill the entire buffer with quotes.
	loops := 0
	for copied := 0; copied < len(qb); loops++ {
		d := w.ToFill()
		assert.True(t, len(d) > 0, "should never return a 0 sized buffer")

		c := copy(d, qb[copied:])
		w.Filled(c)
		copied += c
	}
	assert.Equal(t, len(qb)/32+1, loops)

	// Verify the entire buffer.
	loops = 0
	for verified := 0; verified < len(qb); loops++ {
		d := w.ToEmpty()
		assert.True(t, len(d) > 0, "should never return a 0 sized buffer")
		assert.Equal(t, string(qb[verified:verified+len(d)]), string(d), "diff in loop %d, offset %d", loops, verified)

		// Ensure that we empty the buffer at different steps / sizes.
		step := (loops % len(d)) + 1
		j := 0
		for {
			if j+step > len(d) {
				w.Empty(len(d) - j)
				break
			}

			w.Empty(step)
			j += step
		}
		verified += len(d)
	}
	assert.Equal(t, len(qb)/32+1, loops)

	// Acknowledge roughly half the data.
	verified := 0
	for verified < len(qb)/2 {
		w.Acknowledge(17) // not a multiple of 32, or 41, should challenge different offsets.

		d := w.ToEmpty()
		assert.Equal(t, 0, len(d))
		e := w.ToFill()
		assert.Equal(t, 32, len(e))

		verified += 17
	}

	// Try to reset the acknowledge number.
	err := w.Reset(12)
	assert.NotNil(t, err, "trying to seek back to an already acknowledged byte")
	err = w.Reset(uint32(len(qb) + 12))
	assert.NotNil(t, err, "trying to seek after data in buffer")

	// Reset the state of the buffer, we should have seeked back to the beginning.
	for i := 0; ; i++ {
		verified += 17
		if verified >= len(qb) {
			break
		}
		err = w.Reset(uint32(verified))
		assert.Nil(t, err)
		if err != nil {
			t.Fail()
			break
		}

		inner := verified
		for loops = 0; inner < len(qb); loops++ {
			d := w.ToEmpty()
			assert.True(t, len(d) > 0, "should never return a 0 sized buffer")
			if !assert.Equal(t, string(qb[inner:inner+len(d)]), string(d), "verified %d diff in loop %d, offset %d", verified, loops, inner) {
				break
			}
			w.Empty(len(d))
			if len(d) == 0 {
				t.Fail()
				break
			}

			inner += len(d)
		}
		t.Logf("TOTAL LOOPS %d - verified %d out of %d", loops, verified, len(qb))
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
	data = w.ToEmpty()
	assert.Equal(t, quote, string(data))
	w.Empty(len(data))

	data = w.ToEmpty()
	assert.Equal(t, quote[3:5], string(data))
	w.Empty(len(data))

	data = w.ToFill()
	copy(data, quote)
	w.Filled(len(quote))

	data = w.ToEmpty()
	assert.Equal(t, quote, string(data))
	w.Empty(len(data))
}

func TestReceiveWindowResetComplex(t *testing.T) {
	wrong := "Wrong is wrong, no matter who does it or says it."

	// Build a large buffer we can use as reference.
	qb := []byte{}
	for i := 0; i < 1000; i++ {
		qb = append(qb, []byte(quote)...)
	}

	// Small buffer size used by the window library, so we stress the buffer chaining.
	bsize := 32
	p := NewBufferPool(bsize)
	w := NewReceiveWindow(p)

	// Fill the entire buffer with quotes.
	loops := 0
	for copied := 0; copied < len(qb); loops++ {
		d := w.ToFill()
		assert.True(t, len(d) > 0, "should never return a 0 sized buffer")

		c := copy(d, qb[copied:])
		w.Filled(c)
		copied += c
	}
	assert.Equal(t, len(qb)/32+1, loops)

	// Empty a part of the buffer.
	loops = 0
	emptied := 0
	for ; emptied < len(qb)/2; loops++ {
		d := w.ToEmpty()
		assert.Equal(t, bsize, len(d))
		assert.Equal(t, string(qb[emptied:emptied+len(d)]), string(d), "loop %d", loops)
		emptied += len(d)
		w.Empty(len(d))
	}
	err := w.Reset(53)
	assert.Nil(t, err)

	// The reset should put mechanisms in place so that any further fill of the buffer
	// results in skipping data - it's not written in the buffer until we catch up.
	loops = 0
	stop := len(qb) - 53
	copied := 0
	res := []byte{}
	for ; copied < stop; loops++ {
		d := w.ToFill()
		assert.True(t, len(d) > 0, "should never return a 0 sized buffer - loop %d", loops)
		assert.Equal(t, bsize, len(d))

		c := 0
		c = copy(d, wrong)
		res = append(res, wrong[:c]...)
		w.Filled(c)
		copied += c
	}
	assert.Equal(t, loops*bsize, copied)

	// We emptied half the buffer earlier, but not the rest. We reset the cursor to 53.
	// Now the data that was already filled should still be unchanged, but past that, we
	// should have the new data. Let's check.
	loops = 0
	for ; emptied < len(qb); loops++ {
		d := w.ToEmpty()
		delta := len(qb) - emptied
		if delta > bsize {
			delta = bsize
		}
		assert.Equal(t, delta, len(d), "loop %d, emptied %d, qb %d", loops, emptied, len(qb))
		assert.Equal(t, string(qb[emptied:emptied+len(d)]), string(d), "loop %d", loops)
		emptied += len(d)
		w.Empty(len(d))
	}

	d := w.ToEmpty()
	leftover := (((len(qb)-53)/bsize)+1)*bsize - (len(qb) - 53)
	assert.Equal(t, leftover, len(d), "wrong %d, copied %d, %d", len(wrong), copied, len(qb)-53)
	assert.Equal(t, string(res[len(qb)-53:]), string(d), "%d", (len(qb)-53)%32)
	w.Empty(len(d))
	emptied += len(d)

	d = w.ToEmpty()
	assert.Equal(t, 0, len(d))

	// Reset one more time.
	err = w.Reset(57)
	assert.Nil(t, err)

	// Fill one more time.
	loops = 0
	stop = emptied - 57
	copied = 0
	res = []byte{}
	wrong = "Facts do not cease to exist because they are ignored."
	for ; copied < stop; loops++ {
		d := w.ToFill()
		assert.True(t, len(d) > 0, "should never return a 0 sized buffer - loop %d", loops)
		assert.Equal(t, bsize, len(d))

		c := 0
		c = copy(d, wrong)
		res = append(res, wrong[:c]...)
		w.Filled(c)
		copied += c
	}
	assert.Equal(t, loops*bsize, copied)

	d = w.ToEmpty()
	assert.Equal(t, string(res[stop:]), string(d))
}
