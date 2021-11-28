package token

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var flagIterations = flag.Int("iterations", 10000, "Corrupt iterations")

// Use simple symmetric keyholders.
func TestMultiSimple(t *testing.T) {
	rng := rand.New(rand.NewSource(1))

	// No KeyHolder at all is illegal.
	mke, err := NewMultiKeyCryptoEncoder(rng, SymmetricCreator)
	assert.Error(t, err)
	assert.Nil(t, mke)

	// Configure two keyholders. The data will be decodable with any one of the two.
	k1, err := GenerateSymmetricKey(rng, 0)
	assert.NoError(t, err)
	kh1, err := NewSymmetricEncoder(rng, UseSymmetricKey(k1))
	assert.NoError(t, err)
	k2, err := GenerateSymmetricKey(rng, 0)
	assert.NoError(t, err)
	kh2, err := NewSymmetricEncoder(rng, UseSymmetricKey(k2))
	assert.NoError(t, err)

	mke, err = NewMultiKeyCryptoEncoder(rng, SymmetricCreator, WithKeyHolder(kh1, kh2))
	assert.NoError(t, err)
	assert.NotNil(t, mke)

	sentence := []byte("A country that does not know how to read and write is easy to deceive.")

	encoded, err := mke.Encode(sentence)
	assert.NoError(t, err)
	assert.NotEqual(t, sentence, encoded)
	assert.True(t, len(encoded) > len(sentence))

	// Simple decrypt with the same multi key encoder. Should succeed.
	_, decoded, err := mke.Decode(context.Background(), encoded)
	assert.NoError(t, err)
	assert.Equal(t, sentence, decoded)

	// Decrypt with a single key - see that's enough to decrypt the message.
	// (keyholder recreated from scratch for extra safety)
	kh3, err := NewSymmetricEncoder(rng, UseSymmetricKey(k1))
	assert.NoError(t, err)
	mkd3, err := NewMultiKeyCryptoEncoder(rng, SymmetricCreator, WithKeyHolder(kh3))
	assert.NoError(t, err)

	_, decoded, err = mkd3.Decode(context.Background(), encoded)
	assert.NoError(t, err)
	assert.Equal(t, sentence, decoded)

	kh4, err := NewSymmetricEncoder(rng, UseSymmetricKey(k2))
	assert.NoError(t, err)
	mkd4, err := NewMultiKeyCryptoEncoder(rng, SymmetricCreator, WithKeyHolder(kh4))
	assert.NoError(t, err)

	_, decoded, err = mkd4.Decode(context.Background(), encoded)
	assert.NoError(t, err)
	assert.Equal(t, sentence, decoded)

	// Decrypt with a set of keys, none of which is valid.
	// (keyholder recreated from scratch for extra safety)
	ki1, err := GenerateSymmetricKey(rng, 0)
	assert.NoError(t, err)
	ki2, err := GenerateSymmetricKey(rng, 0)
	assert.NoError(t, err)

	kih1, err := NewSymmetricEncoder(rng, UseSymmetricKey(ki1))
	assert.NoError(t, err)
	kih2, err := NewSymmetricEncoder(rng, UseSymmetricKey(ki2))
	assert.NoError(t, err)

	mki, err := NewMultiKeyCryptoEncoder(rng, SymmetricCreator, WithKeyHolder(kih1, kih2))
	assert.NoError(t, err)
	assert.NotNil(t, mki)

	_, decoded, err = mki.Decode(context.Background(), encoded)
	assert.Error(t, err)

	// Corrupt text randomly and see that deciphering fails.
	// (but first, check that deciphering still works fine)
	_, decoded, err = mkd3.Decode(context.Background(), encoded)
	assert.NoError(t, err)
	assert.Equal(t, sentence, decoded)

	_, decoded, err = mkd4.Decode(context.Background(), encoded)
	assert.NoError(t, err)
	assert.Equal(t, sentence, decoded)

	seed := time.Now().UnixNano()
	t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
		rng = rand.New(rand.NewSource(seed))
		for i := 0; i < *flagIterations; i++ {
			offset := rng.Intn(len(encoded))
			bit := byte(1 << rng.Intn(8))

			t.Run(fmt.Sprintf("%d-offset%d-bit%d", i, offset, bit), func(t *testing.T) {
				newencoded := append([]byte{}, encoded...)
				newencoded[offset] ^= bit

				// Depending on the random bit we pick, we may be corrupting:
				// - the first key - decrypting with the second key will succeed.
				// - the second key - decrypting with the first key will succeed.
				// - the text stored - decrypting with any key will succeed.
				_, _, err3 := mkd3.Decode(context.Background(), newencoded)
				_, _, err4 := mkd4.Decode(context.Background(), newencoded)
				assert.True(t, err3 != nil || err4 != nil, "err3: %v, err4: %v", err3, err4)
			})
		}
	})
}

// Use a mix of keyholders.
func TestMultiMix(t *testing.T) {
	rng := rand.New(rand.NewSource(1))

	// Configure a symmetric encoder. Usable both to encrypt and decrypt.
	k1, err := GenerateSymmetricKey(rng, 0)
	assert.NoError(t, err)
	kh1, err := NewSymmetricEncoder(rng, UseSymmetricKey(k1))
	assert.NoError(t, err)

	// Configure an asymmetric encoder. Can only be used to encrypt!
	pub, priv, err := GenerateAsymmetricKeys(rng)
	assert.NoError(t, err)
	kh2, err := NewAsymmetricEncoder(rng, UsePublicKey(pub))
	assert.NoError(t, err)

	mke, err := NewMultiKeyCryptoEncoder(rng, SymmetricCreator, WithKeyHolder(kh1, kh2))
	assert.NoError(t, err)
	assert.NotNil(t, mke)

	sentence := []byte("Learn what is to be taken seriously and laugh at the rest.")

	encoded, err := mke.Encode(sentence)
	assert.NoError(t, err)
	assert.NotEqual(t, sentence, encoded)
	assert.True(t, len(encoded) > len(sentence))

	// Simple decrypt with the same multi key encoder should succeed,
	// thanks to the symmetric key.
	_, decoded, err := mke.Decode(context.Background(), encoded)
	assert.NoError(t, err)
	assert.Equal(t, sentence, decoded)

	// Verify the symmetric key.
	mksym, err := NewMultiKeyCryptoEncoder(rng, SymmetricCreator, WithKeyHolder(kh1))
	assert.NoError(t, err)
	assert.NotNil(t, mksym)
	_, decoded, err = mksym.Decode(context.Background(), encoded)
	assert.NoError(t, err)
	assert.Equal(t, sentence, decoded)

	// Verify the asymmetric key (should fail, no private key).
	mkasy, err := NewMultiKeyCryptoEncoder(rng, SymmetricCreator, WithKeyHolder(kh2))
	assert.NoError(t, err)
	assert.NotNil(t, mkasy)
	_, decoded, err = mkasy.Decode(context.Background(), encoded)
	assert.Error(t, err)

	// Prepare an asymmetric decipher with both private and public key.
	khpriv, err := NewAsymmetricEncoder(rng, UsePublicKey(pub), UsePrivateKey(priv))
	assert.NoError(t, err)
	mkasy, err = NewMultiKeyCryptoEncoder(rng, SymmetricCreator, WithKeyHolder(khpriv))
	assert.NoError(t, err)
	assert.NotNil(t, mkasy)
	_, decoded, err = mkasy.Decode(context.Background(), encoded)
	assert.NoError(t, err)
	assert.Equal(t, sentence, decoded)
}
