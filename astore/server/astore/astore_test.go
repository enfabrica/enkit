package astore_test

import (
	"github.com/enfabrica/enkit/astore/client/astore"
	astoreServer "github.com/enfabrica/enkit/astore/server/astore"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"testing"
)

func TestSid(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	for i := 0; i < 1000; i++ {
		value, err := astoreServer.GenerateSid(rng)
		assert.Nil(t, err)
		assert.Equal(t, 34, len(value), "value: %s", value)
	}
}

func TestUid(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	for i := 0; i < 1000; i++ {
		value, err := astoreServer.GenerateUid(rng)
		assert.Nil(t, err)
		assert.Equal(t, 32, len(value), "value: %s", value)
		assert.True(t, astore.IsUid(value))
	}
}
