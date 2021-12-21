package token

import (
	"context"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"testing"
)

func TestTypeEncoder(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	be, err := NewSymmetricEncoder(rng, WithGeneratedSymmetricKey(128))
	assert.NotNil(t, be)
	assert.Nil(t, err)

	te := NewTypeEncoder(be)
	assert.NotNil(t, te)

	data1, err := te.Encode("this is a string")
	assert.Nil(t, err)
	data2, err := te.Encode("this is a string")
	assert.Nil(t, err)
	assert.NotEqual(t, data1, data2)

	var result string
	_, err = te.Decode(context.Background(), data1, &result)
	assert.Nil(t, err)
	assert.Equal(t, "this is a string", result)
}
