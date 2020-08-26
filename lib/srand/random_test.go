package srand

import (
	"crypto/rand"
	"encoding/binary"
	mrand "math/rand"
	"testing"
)

func BenchmarkBuffered(b *testing.B) {
	for n := 0; n < b.N; n++ {
		Source.Uint64()
	}
}

func BenchmarkUnbuffered(b *testing.B) {
	v := uint64(0)
	for n := 0; n < b.N; n++ {
		if err := binary.Read(rand.Reader, binary.BigEndian, &v); err != nil {
			b.Errorf("read error: %s", err)
		}
	}
}

func BenchmarkJustRead(b *testing.B) {
	buff := make([]byte, 8)
	for n := 0; n < b.N; n++ {
		if n, err := rand.Read(buff); err != nil || n != 8 {
			b.Errorf("read error: %s", err)
		}
	}
}

func BenchmarkWeakRand(b *testing.B) {
	for n := 0; n < b.N; n++ {
		_ = mrand.Uint64()
	}
}
