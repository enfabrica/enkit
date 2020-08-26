// A thread safe cryptographically secure random number generator.
//
// This is just a tiny wrapper around crypto/rand which:
// - performs buffering - rather than read from /dev/urandom for each
//   random number request, it buffers entropy.
// - uses a thread safe pool - rather than locking and one global state,
//   it maintains a buffer of entropy per thread pool.
//
// If you run the included benchmark, you can see this is order of magnitudes
// faster than just using crypto/rand.
//
// To use it, just create a new random number generator with it:
//
//   rng := rand.New(srand.Source)
//
package srand

import (
	"bufio"
	"crypto/rand"
	"encoding/binary"
	"sync"
)

var pool *sync.Pool

func init() {
	pool = &sync.Pool{
		New: func() interface{} {
			return bufio.NewReaderSize(rand.Reader, 4096)
		},
	}
}

type Generator struct{}

func (s Generator) Seed(seed int64) {}

func (s Generator) Int63() int64 {
	return int64(s.Uint64() & ^uint64(1<<63))
}

func (s Generator) Uint64() (v uint64) {
	reader := pool.Get().(*bufio.Reader)
	if err := binary.Read(reader, binary.BigEndian, &v); err != nil {
		panic(err)
	}
	pool.Put(reader)
	return v
}

var Source = &Generator{}
