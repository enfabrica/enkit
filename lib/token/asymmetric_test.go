package token

import (
	"context"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"testing"
)

func TestAsymmetricSimple(t *testing.T) {
	rng := rand.New(rand.NewSource(1))

	// A public key is mandatory!
	_, err := NewAsymmetricEncoder(rng)
	assert.Error(t, err)

	ae, err := NewAsymmetricEncoder(rng, WithGeneratedAsymmetricKey())
	assert.NoError(t, err)

	text := []byte("When I give food to the poor, they call me a saint. When I ask why the poor have no food, they call me a socialist")

	encoded, err := ae.Encode(text)
	assert.NoError(t, err)
	assert.True(t, len(encoded) > len(text))

	_, original, err := ae.Decode(context.Background(), encoded)
	assert.NoError(t, err)
	assert.Equal(t, original, text)

	// Try operations without a private key.
	a2, err := NewAsymmetricEncoder(rng, UsePublicKey(ae.PublicKey()))
	assert.NoError(t, err)

	// Decoding should fail.
	_, original, err = a2.Decode(context.Background(), encoded)
	assert.Error(t, err)

	// But encoding should succeed.
	text2 := []byte("Despair is typical of those who do not understand the causes of evil, see no way out, and are incapable of struggle")
	encoded, err = a2.Encode(text2)
	assert.NoError(t, err)

	// And can still be decoded with a private key.
	_, original, err = ae.Decode(context.Background(), encoded)
	assert.NoError(t, err)
	assert.Equal(t, text2, original)
}
