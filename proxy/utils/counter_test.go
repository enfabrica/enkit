package utils

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBasicCounter(t *testing.T) {
	var c Counter
	assert.Equal(t, uint64(0), c.Get())

	c.Increment()
	assert.Equal(t, uint64(1), c.Get())

	c.Add(10)
	assert.Equal(t, uint64(11), c.Get())

	c.SetIfGreatest(9)
	assert.Equal(t, uint64(11), c.Get())

	c.SetIfGreatest(13)
	assert.Equal(t, uint64(13), c.Get())
}
