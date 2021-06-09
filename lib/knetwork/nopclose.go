package knetwork

import (
	"io"
)

type nopReadCloser struct {
	io.Reader
}

func (n *nopReadCloser) Close() error {
	return nil
}

// NopReadCloser turns any io.Reader into a io.ReadCloser with a nop closer.
//
// This can also be used to ignore Close() events on another ReadCloser.
func NopReadCloser(reader io.Reader) io.ReadCloser {
	return &nopReadCloser{reader}
}

type nopWriteCloser struct {
	io.Writer
}

func (n *nopWriteCloser) Close() error {
	return nil
}

// NopWriteCloser turns any io.Writer into a io.WriteCloser with a nop closer.
//
// This can also be used to ignore Close() events on another WriteCloser.
func NopWriteCloser(writer io.Writer) io.WriteCloser {
	return &nopWriteCloser{writer}
}
