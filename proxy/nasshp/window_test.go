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

	_, err := w.AcknowledgeUntil(0)
	assert.Nil(t, err)

	_, err = w.AcknowledgeUntil(2)
	assert.NotNil(t, err)

	d := w.ToEmpty()
	assert.Equal(t, 0, len(d))

	d = w.ToFill()
	l := copy(d, quote)
	assert.Equal(t, bsize, l)
	w.Fill(l)

	// Still failing, as the data has not been sent.
	_, err = w.AcknowledgeUntil(2)
	assert.NotNil(t, err)

	// Mark 4 bytes as sent.
	d = w.ToEmpty()
	assert.Equal(t, bsize, len(d))
	w.Empty(4)
	d = w.ToEmpty()
	assert.Equal(t, bsize-4, len(d))

	// Success!
	_, err = w.AcknowledgeUntil(2)
	assert.Nil(t, err)

	// Shouldn't change how much data we have left to empty.
	d = w.ToEmpty()
	assert.Equal(t, bsize-4, len(d))

	// No change if we re-acknowledge the same data.
	_, err = w.AcknowledgeUntil(2)
	assert.Nil(t, err)
	// Error if we go back and un-acknowledge some data.
	_, err = w.AcknowledgeUntil(1)
	assert.NotNil(t, err)

	// Same amount of data to empty.
	d = w.ToEmpty()
	assert.Equal(t, bsize-4, len(d))

	// Acknowledge up to all the data we have.
	_, err = w.AcknowledgeUntil(4)
	assert.Nil(t, err)
	d = w.ToEmpty()
	assert.Equal(t, bsize-4, len(d))

	// Finally, acknowledge everything.
	w.Empty(len(d))
	_, err = w.AcknowledgeUntil(uint32(bsize))
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
		w.Fill(c)
		copied += c
	}
	assert.Equal(t, len(qb)/bsize+1, loops)

	// Verify the entire buffer.
	loops = 0
	verified := 0
	for ; verified < len(qb); loops++ {
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
	assert.Equal(t, len(qb)/bsize+1, loops)

	// Acknowledge roughly half the data.
	acknowledged := 0
	for acknowledged < len(qb)/2 {
		w.Acknowledge(17) // not a multiple of bsize, or 41, should challenge different offsets.

		d := w.ToEmpty()
		assert.Equal(t, 0, len(d))
		e := w.ToFill()
		assert.Equal(t, bsize-(len(qb)%bsize), len(e))

		acknowledged += 17
	}

	// Try to reset the acknowledge number.
	err := w.Reset(12)
	assert.NotNil(t, err, "trying to seek back to an already acknowledged byte")
	err = w.Reset(uint32(len(qb) + 12))
	assert.NotNil(t, err, "trying to seek after data in buffer")

	assert.Equal(t, uint64(len(qb)), w.Filled)
	assert.Equal(t, uint64(verified), w.Emptied)
	assert.Equal(t, uint64(acknowledged), w.acknowledged)

	acknowledged += 17
	err = w.Reset(uint32(acknowledged))
	assert.Nil(t, err)

	assert.Equal(t, uint64(len(qb)), w.Filled, "filled")
	assert.Equal(t, uint64(acknowledged), w.Emptied, "emptied")
	assert.Equal(t, uint64(acknowledged), w.acknowledged, "acknowledged")
	w.Empty(17)

	// Reset the state of the buffer, we should have seeked back to the beginning.
	for i := 0; ; i++ {
		acknowledged += 17
		if acknowledged >= len(qb) {
			break
		}
		err = w.Reset(uint32(acknowledged))
		assert.Nil(t, err)
		if err != nil {
			t.Fail()
			break
		}

		inner := acknowledged
		for loops = 0; inner < len(qb); loops++ {
			d := w.ToEmpty()
			assert.True(t, len(d) > 0, "should never return a 0 sized buffer - %d out of %d", inner, len(qb))
			if !assert.Equal(t, string(qb[inner:inner+len(d)]), string(d), "acknowledged %d diff in loop %d, offset %d", acknowledged, loops, inner) {
				break
			}
			w.Empty(len(d))
			if len(d) == 0 {
				t.Fail()
				break
			}

			inner += len(d)
		}
		assert.Equal(t, w.Emptied, w.Filled)
		assert.Equal(t, uint64(acknowledged), w.acknowledged, "acknowledged")

		t.Logf("%d TOTAL LOOPS %d - acknowledged %d out of %d", i, loops, acknowledged, len(qb))
	}
}

func TestReceiveWindowSimple(t *testing.T) {
	bsize := 8192
	p := NewBufferPool(bsize)
	w := NewReceiveWindow(p)

	data := w.ToFill()
	assert.Equal(t, bsize, len(data))
	copy(data, quote)
	w.Fill(len(quote))

	data = w.ToFill()
	assert.Equal(t, bsize-len(quote), len(data))
	copy(data, quote)
	w.Fill(len(quote))

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
		w.Fill(l)
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
	w.Fill(l)

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
	assert.Equal(t, w.Filled, uint64(len(quote)))
	assert.Equal(t, w.Emptied, uint64(0))

	// Now we really moved back the needle.
	err = w.Reset(33)
	assert.Nil(t, err)

	// Still, we have the same data to write out, as no more data arrived.
	e = w.ToEmpty()
	assert.Equal(t, len(quote), len(e))
	// And nothing has changed in terms of how many bytes have been filled and emptied.
	assert.Equal(t, w.Filled, uint64(len(quote)))
	assert.Equal(t, w.Emptied, uint64(0))

	// Let's fill in some data (this would be skipped)
	w.Fill(5)
	e = w.ToEmpty()
	assert.Equal(t, len(quote), len(e))

	// Part of this write should be skipped.
	data = w.ToFill()
	copy(data, quote[0:5])
	w.Fill(5)

	// Let's consume the buffer.
	data = w.ToEmpty()
	assert.Equal(t, quote, string(data))
	w.Empty(len(data))

	data = w.ToEmpty()
	assert.Equal(t, quote[3:5], string(data))
	w.Empty(len(data))

	data = w.ToFill()
	copy(data, quote)
	w.Fill(len(quote))

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
		w.Fill(c)
		copied += c
	}

	assert.Equal(t, len(qb)/bsize+1, loops)
	assert.Equal(t, w.Filled, uint64(len(qb)))
	assert.Equal(t, w.Emptied, uint64(0))
	assert.Equal(t, w.reset, uint64(0))

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

	assert.Equal(t, w.Filled, uint64(len(qb)))
	assert.Equal(t, w.Emptied, uint64(emptied))
	assert.Equal(t, w.reset, uint64(0))

	const resetValue = 53
	err := w.Reset(resetValue)
	assert.Nil(t, err)

	// The reset should put mechanisms in place so that any further fill of the buffer
	// results in skipping data - it's not written in the buffer until we catch up.
	loops = 0
	stop := len(qb) - resetValue
	copied := 0
	res := []byte{}
	for ; copied < stop; loops++ {
		d := w.ToFill()
		assert.True(t, len(d) > 0, "should never return a 0 sized buffer - loop %d", loops)
		assert.Equal(t, bsize, len(d))

		c := copy(d, wrong)
		res = append(res, wrong[:c]...)
		w.Fill(c)
		copied += c
	}
	assert.Equal(t, loops*bsize, copied)

	assert.Equal(t, w.Filled, uint64(loops*bsize)+resetValue, "extra %x %x", resetValue+loops*bsize, resetValue+copied-len(qb))
	assert.Equal(t, w.Emptied, uint64(emptied))
	assert.Equal(t, w.reset, uint64(0))

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
	leftover := (((len(qb)-resetValue)/bsize)+1)*bsize - (len(qb) - resetValue)
	assert.Equal(t, leftover, len(d), "wrong %d, copied %d, %d", len(wrong), copied, len(qb)-resetValue)
	assert.Equal(t, string(res[len(qb)-resetValue:]), string(d), "%d", (len(qb)-resetValue)%32)
	w.Empty(len(d))
	emptied += len(d)

	d = w.ToEmpty()
	assert.Equal(t, 0, len(d))
	assert.Equal(t, w.Filled, w.Emptied)
	assert.Equal(t, w.Filled, uint64(emptied))

	// Reset one more time.
	err = w.Reset(57)
	assert.Nil(t, err)
	assert.Equal(t, w.Filled, w.Emptied)

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
		w.Fill(c)
		copied += c
	}
	assert.Equal(t, loops*bsize, copied)
	assert.Equal(t, w.Filled, uint64(copied+57))

	d = w.ToEmpty()
	assert.Equal(t, copied-stop, len(d), "%#v %#v", *w.buffer.First(), *w.buffer.Last())
	assert.Equal(t, w.Filled, w.Emptied+uint64(len(d)), "len d %d", len(d))
	assert.Equal(t, string(res[stop:]), string(d))
}
