package token

import (
	"context"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"testing"
)

func TestSymmetric(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	be, err := NewSymmetricEncoder(rng)
	assert.Nil(t, be)
	assert.NotNil(t, err)

	be, err = NewSymmetricEncoder(rng, WithGeneratedSymmetricKey(12))
	assert.Nil(t, be)
	assert.NotNil(t, err)

	be, err = NewSymmetricEncoder(rng, WithGeneratedSymmetricKey(128))
	assert.NotNil(t, be)
	assert.Nil(t, err)

	data, err := be.Encode([]byte{1, 2, 3, 4})
	assert.Nil(t, err)
	_, original, err := be.Decode(context.Background(), data)
	assert.Nil(t, err)
	assert.Equal(t, []byte{1, 2, 3, 4}, original)
}
