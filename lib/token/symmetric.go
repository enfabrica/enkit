package token

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"io/ioutil"
	"math/rand"
)

type SymmetricEncoder struct {
	rng *rand.Rand

	key    []byte
	cipher cipher.AEAD
}

type SymmetricSetter func(*SymmetricEncoder) error

// UseSymmetricKey uses a key supplied as an array of bytes.
//
// The key can be any array of bytes long enough for the block cipher
// selected. Use GenerateSymmetricKey to create one.
func UseSymmetricKey(key []byte) SymmetricSetter {
	return func(be *SymmetricEncoder) error {
		be.key = key
		return nil
	}
}

// GenerateSymmetricKey generates a new symmetric key.
//
// size is the size of the key to generate in bits. If 0, defaults to 256.
// The only valid values are those accepted by the underlying AES cipher:
// 128, 192 or 256.
func GenerateSymmetricKey(rng *rand.Rand, size int) ([]byte, error) {
	if size == 0 {
		size = 256
	}
	if size != 128 && size != 192 && size != 256 {
		return nil, fmt.Errorf("key size is invalid")
	}
	key := make([]byte, size/8)

	n, err := rng.Read(key)
	if err != nil {
		return nil, err
	}
	if n != int(size/8) {
		return nil, fmt.Errorf("PRNG could not provide %d bytes of key", size)
	}

	return key, nil
}

// Creates a new random key and stores it in settings, or return error.
//
// size represents the key size in bits. If size is 0, uses a default key size
// of 256 bits.
func WithGeneratedSymmetricKey(size int) SymmetricSetter {
	return func(be *SymmetricEncoder) error {
		key, err := GenerateSymmetricKey(be.rng, size)
		if err != nil {
			return err
		}
		return UseSymmetricKey(key)(be)
	}
}

// Reads a key from a file, or creates a new one and stores it in a file.
// Returns error if it can't succeed in generating or storing a new key.
//
// size is the size in bits of the desired key. If left to 0, defaults to
// 256 bits.
//
// The generated (or read) file is just the raw content of the key. For
// example, for a key of 256 bits, it will generate a file of exactly
// 32 bytes, containing the binary encoded key.
func ReadOrGenerateSymmetricKey(path string, size int) SymmetricSetter {
	if size == 0 {
		size = 256
	}

	return func(be *SymmetricEncoder) error {
		var err error
		var key []byte
		if path != "" {
			key, err = ioutil.ReadFile(path)
		}
		if err != nil || len(key) <= 0 || len(key)*8 != size {
			if err := WithGeneratedSymmetricKey(size)(be); err != nil {
				return err
			}
			if path != "" {
				err = ioutil.WriteFile(path, be.key, 0600)
			}
		} else {
			err = UseSymmetricKey(key)(be)
		}
		return err
	}
}

// NewSymmetricEncoder creates a new encoder using AES in GCM mode as a symmetric cipher.
//
// A typical way to use the encoder would be:
//
//    rng := rand.New(srand.Source)  // using github.com/enkit/lib/srand library.
//    ...
//    be, err := NewSymmetricEncoder(rng, ReadOrGenerateSymmetricKey("/etc/keys/connect.key", 0))
//    if err != nil ...
//
// Or:
//
//    key, err := GenerateSymmetricKey(rng, 0)
//    if err != nil ...
//
//    be, err := NewSymmetricEncoder(rng, UseSymmetricKey(key))
//    ...
//
// followed by calls to Encode() and Decode().
func NewSymmetricEncoder(rng *rand.Rand, setters ...SymmetricSetter) (*SymmetricEncoder, error) {
	be := &SymmetricEncoder{rng: rng}
	for _, setter := range setters {
		if err := setter(be); err != nil {
			return nil, err
		}
	}

	if len(be.key) <= 0 {
		return nil, fmt.Errorf("No key set")
	}

	block, err := aes.NewCipher(be.key)
	if err != nil {
		return nil, err
	}

	be.cipher, err = cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return be, nil
}

// SymmetricCreator is a CryptoFactory suitable for use with NewMultiKeyCryptoEncoder().
var SymmetricCreator CryptoFactory = func(rng *rand.Rand, key []byte) (BinaryEncoder, []byte, error) {
	if key != nil {
		be, err := NewSymmetricEncoder(rng, UseSymmetricKey(key))
		return be, key, err
	}

	key, err := GenerateSymmetricKey(rng, 0)
	if err != nil {
		return nil, nil, err
	}

	be, err := NewSymmetricEncoder(rng, UseSymmetricKey(key))
	return be, key, err
}

func (t *SymmetricEncoder) Encode(data []byte) ([]byte, error) {
	nonce := make([]byte, t.cipher.NonceSize())
	if t.rng == nil {
		return nil, fmt.Errorf("no rng - cannot encode")
	}
	n, err := t.rng.Read(nonce)
	if err != nil || n != t.cipher.NonceSize() {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("attempted to generate a nonce of %d bytes, got %d", t.cipher.NonceSize(), n)
	}

	ciphertext := t.cipher.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

func (t *SymmetricEncoder) Decode(ctx context.Context, ciphertext []byte) (context.Context, []byte, error) {
	if len(ciphertext) < t.cipher.NonceSize() {
		return ctx, nil, fmt.Errorf("ciphertext too short to contain nonce")
	}

	nonce := ciphertext[:t.cipher.NonceSize()]
	ciphertext = ciphertext[t.cipher.NonceSize():]

	plaintext, err := t.cipher.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return ctx, nil, err
	}
	return ctx, plaintext, nil
}
