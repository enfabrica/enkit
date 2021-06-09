package knetwork

import (
	"io"
)

type nopWriteCloser struct {
	io.Writer
}

func (n *nopWriteCloser) Close() error {
	return nil
}

// NopWriteCloser turns any io.Writer into a io.WriteCloser with a nop closer.
//
// This can also be used to ignore Close() events on another WriteCloser.
// If you need a NopCloser for read, you can use io.NopCloser.
//
// IMPORTANT: discarding a Close() will of course result in the file remaining
//            open, and corresponding buffers not being flushsed. Be careful
//            when using this.
func NopWriteCloser(writer io.Writer) io.WriteCloser {
	return &nopWriteCloser{writer}
}
