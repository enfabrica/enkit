// Package delimited allows for reading of a stream of protobuf messages
// delimited by size. See
// https://developers.google.com/protocol-buffers/docs/techniques#streaming for
// more info.
package delimited

import (
	"bufio"
	"encoding/binary"
	"io"
)

// Reader wraps an io.Reader and exposes methods to iterate on individual
// messages.
type Reader struct {
	buf *bufio.Reader
}

// NewReader returns a Reader that reads bytes from the supplied Reader.
func NewReader(r io.Reader) *Reader {
	return &Reader{
		buf: bufio.NewReader(r),
	}
}

// Next returns a byte slice with the next message, or an error:
// * io.EOF if there are no messages remaining
// * io.ErrUnexpectedEOF if the stream is corrupted
func (r *Reader) Next() ([]byte, error) {
	size, err := binary.ReadUvarint(r.buf)
	if err != nil {
		return nil, err // Error might be EOF; don't annotate it
	}
	data := make([]byte, size)
	if _, err := io.ReadFull(r.buf, data); err != nil {
		return nil, err
	}
	return data, nil
}
