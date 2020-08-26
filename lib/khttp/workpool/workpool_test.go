package workpool

import (
	"github.com/stretchr/testify/assert"
	"sync/atomic"
	"testing"
)

func TestWorkPool(t *testing.T) {
	wp, err := New(WithWorkers(2), WithQueueSize(3), WithImmediateQueueSize(2))
	assert.Nil(t, err)

	called := int32(0)
	inc := func() {
		atomic.AddInt32(&called, 1)
	}
	wp.Add(inc)
	wp.Add(inc)
	wp.Wait()

	assert.Equal(t, int32(2), atomic.LoadInt32(&called))
	for i := 0; i < 100; i++ {
		wp.Add(inc)
		wp.AddImmediate(inc)
	}
	wp.Wait()
	assert.Equal(t, int32(202), atomic.LoadInt32(&called))
	wp.Done()
}
