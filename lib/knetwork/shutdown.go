package knetwork

import (
	"io"
)

// ReadOnlyCloser is any object that is capable of closing the
// read direction without closing the write direction.
//
// This is typical of eg, TCP connection where a shutdown() call
// can close one direction but not the other.
type ReadOnlyCloser interface {
	io.Reader
	CloseRead() error
}

// WriteOnlyCloser is any object that is capable of closing the
// write direction without closing the read direction.
//
// This is typical of eg, TCP connection where a shutdown() call
// can close one direction but not the other.
type WriteOnlyCloser interface {
	io.Writer
	CloseWrite() error
}

type readOnlyCloseAdapter struct {
	ReadOnlyCloser
}
func (roca *readOnlyCloseAdapter) Close() error {
	return roca.CloseRead()
}

// ReadOnlyClose returns an io.ReadCloser that will only close
// the read channel of the connection passed.
//
// When the object is "Close()", "ReadClose()" will be invoked instead.
func ReadOnlyClose(woc ReadOnlyCloser) io.ReadCloser {
	return &readOnlyCloseAdapter{woc}
}

type writeOnlyCloseAdapter struct {
	WriteOnlyCloser
}
func (roca *writeOnlyCloseAdapter) Close() error {
	return roca.CloseWrite()
}

// WriteOnlyClose returns an io.WriteCloser that will only close
// the write channel of the connection passed.
//
// When the object is "Close()", "WriteClose()" will be invoked instead.
func WriteOnlyClose(woc WriteOnlyCloser) io.WriteCloser {
	return &writeOnlyCloseAdapter{woc}
}
