package token

import (
	"github.com/stretchr/testify/assert"
	"math/rand"
	"testing"
	"time"
	"context"
)

func TestSimple(t *testing.T) {
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

func TestTimeEncoder(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	be, err := NewSymmetricEncoder(rng, WithGeneratedSymmetricKey(128))
	assert.NotNil(t, be)
	assert.Nil(t, err)

	ts := time.Now()
	te := NewChainedEncoder(be, NewTimeEncoder(func() time.Time {
		return ts
	}, time.Second*5))
	assert.NotNil(t, te)

	data, err := te.Encode([]byte{0, 1, 2, 3, 4})
	assert.Nil(t, err)
	assert.NotNil(t, data)

	ts = ts.Add(time.Second * 2)
	_, arr, err := te.Decode(context.Background(), data)
	assert.Nil(t, err)
	assert.NotNil(t, arr)
	assert.Equal(t, []byte{0, 1, 2, 3, 4}, arr)

	// Now the timer has expired.
	ts = ts.Add(time.Second * 3)
	_, arr, err = te.Decode(context.Background(), data)
	assert.NotNil(t, err)
}
