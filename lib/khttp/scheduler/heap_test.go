package scheduler

import (
	"container/heap"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestHeap(t *testing.T) {
	h := &eventHeap{}

	heap.Push(h, &event{when: time.Unix(100, 0)})
	heap.Push(h, &event{when: time.Unix(0, 0)})
	heap.Push(h, &event{when: time.Unix(1000, 0)})
	heap.Push(h, &event{when: time.Unix(10, 0)})

	assert.Equal(t, int64(0), (*h)[0].when.Unix())
	assert.Equal(t, int64(0), heap.Pop(h).(*event).when.Unix())
	assert.Equal(t, int64(10), (*h)[0].when.Unix())
	assert.Equal(t, int64(10), heap.Pop(h).(*event).when.Unix())
	assert.Equal(t, int64(100), (*h)[0].when.Unix())
	assert.Equal(t, int64(100), heap.Pop(h).(*event).when.Unix())
	assert.Equal(t, int64(1000), (*h)[0].when.Unix())
	assert.Equal(t, int64(1000), heap.Pop(h).(*event).when.Unix())
}
