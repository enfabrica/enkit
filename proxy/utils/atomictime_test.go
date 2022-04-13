package utils

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestBasicAtomicTime(t *testing.T) {
	var a AtomicTime
	assert.Equal(t, int64(0), a.Nano())
	a.Reset()
	assert.Equal(t, int64(0), a.Nano())

	now := time.Now()
	a.Set(now)
	assert.Equal(t, now.UnixNano(), a.Nano())
	a.Reset()
	assert.Equal(t, int64(0), a.Nano())
}
