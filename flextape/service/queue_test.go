package service

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"testing"
)

// Verify that methods don't break/crash on empty queues.
func TestQueueEmpty(t *testing.T) {
	iq := invocationQueue{}
	assert.Nil(t, iq.Dequeue())
	assert.Nil(t, iq.Dequeue())
	assert.Equal(t, 0, iq.Len())

	inv, pos := iq.Get("0")
	assert.Nil(t, inv)
	assert.Equal(t, Position(0), pos)
	assert.Nil(t, iq.Forget("0"))
}

func TestQueueOrdering(t *testing.T) {
	iq := invocationQueue{}
	for i := 10; i > 0; i-- {
		iq.Enqueue(&invocation{ID: fmt.Sprintf("id-%02d", i)})
	}

	// QueueID always matches the queue order. Verify it.
	assert.Equal(t, 10, iq.Len())
	for i := 0; i < 10; i++ {
		assert.Equal(t, QueueID(i+1), iq[i].QueueID)
	}

	// Reorder the queue. This should update the QueueID to match the queue order,
	// but leave the Priority and ID unchanged.
	iq.Sort(func(a, b *invocation) bool {
		return a.ID < b.ID
	})
	for i := 0; i < 10; i++ {
		assert.Equal(t, QueueID(i+1), iq[i].QueueID)
		assert.Equal(t, fmt.Sprintf("id-%02d", i+1), iq[i].ID)
	}

	// One more time, with a random seed/random order.
	// This will cause Swap() to be invoked on some entries but not others.
	for i := 0; i < 10; i++ {
		name := rand.Uint64()
		iq[i].ID = fmt.Sprintf("id-%d", name)
	}
	iq.Sort(func(a, b *invocation) bool {
		return a.ID < b.ID
	})
	for i := 0; i < 10; i++ {
		assert.Equal(t, QueueID(i+1), iq[i].QueueID)

		inv, pos := iq.Get(iq[i].ID)
		assert.Equal(t, inv, iq[i])
		assert.Equal(t, Position(i+1), pos)
		assert.Equal(t, Position(i+1), iq.Position(inv))
	}

	// Delete a few elements.
	inv := iq.Forget(iq[3].ID)
	assert.Equal(t, QueueID(0), inv.QueueID)
	inv = iq.Forget(iq[7].ID)
	assert.Equal(t, QueueID(0), inv.QueueID)

	// Check that invariants are maintained in the face of deletions.
	assert.Equal(t, 8, iq.Len())
	for i := 0; i < iq.Len(); i++ {
		assert.Equal(t, QueueID(i+1), iq[i].QueueID)

		inv, pos := iq.Get(iq[i].ID)
		assert.Equal(t, inv, iq[i])
		assert.Equal(t, Position(i+1), pos)
		assert.Equal(t, Position(i+1), iq.Position(inv))
	}
}

// Add/Remove items at random in the queue, verify that all invariants are maintained.
func TestQueueIDs(t *testing.T) {
	iq := invocationQueue{}
	l := 0

	for i := 0; i < 1000; i++ {
		switch r := rand.Intn(5); r {
		case 0:
			fallthrough
		case 1:
			fallthrough
		case 2:
			iq.Enqueue(&invocation{ID: fmt.Sprintf("id-%d", i)})
			l++

		case 3:
			iq.Dequeue()
			if l > 0 {
				l--
			}

		case 4:
			if iq.Len() == 0 {
				continue
			}

			one := rand.Intn(iq.Len())
			iq.Forget(iq[one].ID)
			if l > 0 {
				l--
			}
		}

		assert.Equal(t, l, iq.Len())
		for i := 0; i < iq.Len(); i++ {
			inv, pos := iq.Get(iq[i].ID)
			assert.Equal(t, inv, iq[i])
			assert.Equal(t, Position(i+1), pos)
			assert.Equal(t, Position(i+1), iq.Position(inv))
		}

	}
}
