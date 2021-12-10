package token

import (
	"context"
	"fmt"
	"math/rand"

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
