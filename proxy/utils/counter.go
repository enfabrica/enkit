package utils

import (
	"sync/atomic"
)

type Counter uint64

func (c *Counter) Increment() {
	atomic.AddUint64((*uint64)(c), 1)
}

func (c *Counter) Get() uint64 {
	return atomic.LoadUint64((*uint64)(c))
}
