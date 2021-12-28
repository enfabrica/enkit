package echo

import (
	"bufio"
	"github.com/stretchr/testify/assert"
	"io"
	"net"
	"testing"
)

func TestEcho(t *testing.T) {
	e, err := New("127.0.0.1:0")
	assert.NoError(t, err)
	assert.NotNil(t, e)

	address, err := e.Address()
	assert.NoError(t, err)
	go e.Run()
	defer e.Close()

	c, err := net.Dial("tcp", address.String())
	defer c.Close()
	assert.NoError(t, err)

	// This code is technically incorrect: there is no guarantee
	// that "statement" will fit in a TCP buffer.
	//
	// If the TCP buffer is too small, the WriteString() may block until
	// some bytes have been consumed by the ReadString(), which will never
	// happen and cause a deadlock, as the reader and writer are in the
	// same thread.
	//
	// This is however extremely unlikely (impossible?) to happen in
	// practice as linux defaults to a minimum buffer of a page size
	// 4kb, and (1) the sentence is significantly smaller than that,
	// and (2) this is over loopback, with larger buffers and no loss.
	//
	// There's also a significant gain in simplicity by not using
	// multiple goroutines here.
	statement := "behold of the underminer!\n"
	for i := 0; i < 100; i++ {
		l, err := io.WriteString(c, statement)
		assert.NoError(t, err)
		assert.Equal(t, len(statement), l)

		rback, err := bufio.NewReader(c).ReadString('\n')
		assert.NoError(t, err)
		assert.Equal(t, rback, statement)
	}
}
