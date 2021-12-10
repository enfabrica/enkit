package token

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/rand"

	"golang.org/x/crypto/nacl/box"
	"golang.org/x/crypto/nacl/sign"
)

type VerifyingKey [32]byte

func (pk *VerifyingKey) ToBytes() *[32]byte {
	return (*[32]byte)(pk)
}

func VerifyingKeyFromSlice(slice []byte) (*VerifyingKey, error) {
	parsed := VerifyingKey{}
	if copy(parsed[:], slice) != len(parsed) || len(slice) > len(parsed) {
		return nil, fmt.Errorf("invalid signing key length - must be %d bytes", len(parsed))
	}
	return &parsed, nil
}

type SigningKey [64]byte

func SigningKeyFromSlice(slice []byte) (*SigningKey, error) {
	parsed := SigningKey{}
	if copy(parsed[:], slice) != len(parsed) || len(slice) > len(parsed) {
		return nil, fmt.Errorf("invalid signing key length - must be %d bytes", len(parsed))
	}
	return &parsed, nil
}

func (pk *SigningKey) ToBytes() *[64]byte {
	return (*[64]byte)(pk)
}

// SigningEncoder is an encoder that adds a cryptographically strong signature to the data.
//
// Data will fail to decode if the signature is invalid.
type SigningEncoder struct {
	rng       *rand.Rand
	signing   *SigningKey
	verifying *VerifyingKey
}

type SigningSetter func(*SigningEncoder) error

func UseSigningKey(signing *SigningKey) SigningSetter {
	return func(be *SigningEncoder) error {
		be.signing = signing
		return nil
	}
}

func UseVerifyingKey(verify *VerifyingKey) SigningSetter {
	return func(be *SigningEncoder) error {
		be.verifying = verify
		return nil
	}
}

func GenerateSigningKey(rng *rand.Rand) (*VerifyingKey, *SigningKey, error) {
	pub, priv, err := sign.GenerateKey(rng)
	if err != nil {
		return nil, nil, err
	}
	return (*VerifyingKey)(pub), (*SigningKey)(priv), nil
}

func NewSigningEncoder(rng *rand.Rand, setters ...SigningSetter) (*SigningEncoder, error) {
	be := &SigningEncoder{rng: rng}
	for _, setter := range setters {
		if err := setter(be); err != nil {
			return nil, err
		}
	}

	if be.signing == nil && be.verifying == nil {
		return nil, fmt.Errorf("neither a signing nor verifying key has been provided - at least one of the two must be supplied")
	}

	return be, nil
}

func (t *SigningEncoder) Encode(data []byte) ([]byte, error) {
	if t.signing == nil {
		return nil, fmt.Errorf("a signing key must be supplied to encode data")
	}
	return sign.Sign(nil, data, t.signing.ToBytes()), nil
}

func (t *SigningEncoder) Decode(ctx context.Context, value []byte) (context.Context, []byte, error) {
	if t.verifying == nil {
		return ctx, nil, fmt.Errorf("a verifying key must be supplied to decode data")
	}
	data, ok := sign.Open(nil, value, t.verifying.ToBytes())
	if !ok {
		return ctx, nil, fmt.Errorf("signature did not match")
	}
	return ctx, data, nil
}

const AsymmetricKeyLength = 32

type AsymmetricKey [AsymmetricKeyLength]byte

func (k *AsymmetricKey) ToByte() *[AsymmetricKeyLength]byte {
	return (*[AsymmetricKeyLength]byte)(k)
}

const AsymmetricNonceLength = 24

type AsymmetricNonce [AsymmetricNonceLength]byte

func (n *AsymmetricNonce) ToByte() *[AsymmetricNonceLength]byte {
	return (*[AsymmetricNonceLength]byte)(n)
}

func AsymmetricKeyFromSlice(key []byte) (*AsymmetricKey, error) {
	parsedAsymmetricKey := AsymmetricKey{}
	if copy(parsedAsymmetricKey[:], key) != AsymmetricKeyLength || len(key) > AsymmetricKeyLength {
		return nil, fmt.Errorf("invalid key length - must be %d bytes", AsymmetricKeyLength)
	}
	return &parsedAsymmetricKey, nil
}

func AsymmetricKeyFromString(key string) (*AsymmetricKey, error) {
	return AsymmetricKeyFromSlice([]byte(key))
}

func AsymmetricKeyFromHex(key string) (*AsymmetricKey, error) {
	slice, err := hex.DecodeString(key)
	if err != nil {
		return nil, err
	}
	return AsymmetricKeyFromSlice(slice)
}

func AsymmetricNonceFromSlice(nonce []byte) (*AsymmetricNonce, error) {
	parsedAsymmetricNonce := AsymmetricNonce{}
	if copy(parsedAsymmetricNonce[:], nonce) != AsymmetricNonceLength || len(nonce) > AsymmetricNonceLength {
		return nil, fmt.Errorf("invalid nonce length - must be %d bytes", AsymmetricNonceLength)
	}
	return &parsedAsymmetricNonce, nil
}

// AsymmetricEncoder is a BinaryEncoder capable of encrypting data with a public key
// and decoding it using a public and private key pair.
//
// AsymmetricEncoder is based on the naccl library, it is pretty much an interface
// adapter around the OpenAnonymous and SealAnonymous functions.
type AsymmetricEncoder struct {
	rng       *rand.Rand
	priv, pub *AsymmetricKey
}

type AsymmetricSetter func(*AsymmetricEncoder) error

func UsePrivateKey(key *AsymmetricKey) AsymmetricSetter {
	return func(be *AsymmetricEncoder) error {
		be.priv = key
		return nil
	}
}

func UsePublicKey(key *AsymmetricKey) AsymmetricSetter {
	return func(be *AsymmetricEncoder) error {
		be.pub = key
		return nil
	}
}

func UseKeyPair(pub, priv *AsymmetricKey) AsymmetricSetter {
	return func(be *AsymmetricEncoder) error {
		if err := UsePublicKey(pub)(be); err != nil {
			return err
		}
		return UsePrivateKey(priv)(be)
	}
}

// GenerateAsymmetricKeys generates a public key and private key.
func GenerateAsymmetricKeys(rng *rand.Rand) (*AsymmetricKey, *AsymmetricKey, error) {
	pub, priv, err := box.GenerateKey(rng)
	return (*AsymmetricKey)(pub), (*AsymmetricKey)(priv), err
}

// Creates a new random public and private key, or return error.
func WithGeneratedAsymmetricKey() AsymmetricSetter {
	return func(be *AsymmetricEncoder) error {
		pub, priv, err := GenerateAsymmetricKeys(be.rng)
		if err != nil {
			return err
		}
		return UseKeyPair(pub, priv)(be)
	}
}

func NewAsymmetricEncoder(rng *rand.Rand, setters ...AsymmetricSetter) (*AsymmetricEncoder, error) {
	be := &AsymmetricEncoder{rng: rng}
	for _, setter := range setters {
		if err := setter(be); err != nil {
			return nil, err
		}
	}

	if be.pub == nil {
		return nil, fmt.Errorf("a public key MUST always be provided")
	}
	return be, nil
}

func (t *AsymmetricEncoder) Encode(data []byte) ([]byte, error) {
	return box.SealAnonymous(nil, data, t.pub.ToByte(), t.rng)
}

func (t *AsymmetricEncoder) Decode(ctx context.Context, ciphertext []byte) (context.Context, []byte, error) {
	if t.priv == nil {
		return ctx, nil, fmt.Errorf("a private key MUST be provided to decode the data")
	}
	original, ok := box.OpenAnonymous(nil, ciphertext, t.pub.ToByte(), t.priv.ToByte())
	if !ok {
		return ctx, nil, fmt.Errorf("error deciphering asymmetric data")
	}
	return ctx, original, nil
}

func (t *AsymmetricEncoder) PrivateKey() *AsymmetricKey {
	return t.priv
}

func (t *AsymmetricEncoder) PublicKey() *AsymmetricKey {
	return t.pub
}
