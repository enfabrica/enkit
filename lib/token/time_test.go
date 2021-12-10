package token

import (
	"context"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"testing"
	"time"
)

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
