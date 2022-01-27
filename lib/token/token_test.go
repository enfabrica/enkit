package token

import (
	"context"
	"github.com/enfabrica/enkit/lib/config/marshal"
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

func TestTypeEncoderMarshal(t *testing.T) {
	be := NewBase64UrlEncoder()
	assert.NotNil(t, be)

	tgob := NewTypeEncoder(be)
	assert.NotNil(t, tgob)

	tyaml := NewTypeEncoder(be, WithMarshaller(marshal.Yaml))
	assert.NotNil(t, tyaml)

	data := "When morality comes up against profit, it is seldom that profit loses."

	result1, err := tgob.Encode(data)
	assert.Nil(t, err)
	result2, err := tgob.Encode(data)
	assert.Nil(t, err)
	result3, err := tyaml.Encode(data)

	// This is just to verify that there is no entropy accidentally added.
	assert.Equal(t, result1, result2)
	assert.NotEqual(t, result1, result3)

	var decoded string
	_, err = tyaml.Decode(context.Background(), result3, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, data, decoded)
}
