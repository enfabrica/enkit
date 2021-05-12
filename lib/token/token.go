package token

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"golang.org/x/crypto/nacl/sign"
	"io/ioutil"
	"math/rand"
	"time"
	)

// BinaryEncoders convert an array of bytes into another by applying binary transformations.
// For example: encryption, signature, ...
type BinaryEncoder interface {
	Encode([]byte) ([]byte, error)
	Decode([]byte) ([]byte, error)
}

type ChainedEncoder []BinaryEncoder

func NewChainedEncoder(enc ...BinaryEncoder) *ChainedEncoder {
	return (*ChainedEncoder)(&enc)
}

func (ce *ChainedEncoder) Encode(data []byte) ([]byte, error) {
	encs := ([]BinaryEncoder)(*ce)
	for _, enc := range encs {
		var err error
		data, err = enc.Encode(data)
		if err != nil {
			return nil, err
		}
	}
	return data, nil
}

func (ce *ChainedEncoder) Decode(data []byte) ([]byte, error) {
	encs := ([]BinaryEncoder)(*ce)
	for ix := range encs {
		enc := encs[len(encs)-ix-1]

		var err error
		data, err = enc.Decode(data)
		if err != nil {
			return nil, err
		}
	}
	return data, nil
}

// StringEncoders convert an array of bytes into a string safe for specific applications.
// For example: mime64, url, ...
type StringEncoder interface {
	Encode([]byte) (string, error)
	Decode(string) ([]byte, error)
}

type TimeSource func() time.Time

type TimeEncoder struct {
	validity time.Duration
	now      TimeSource
}

func NewTimeEncoder(source TimeSource, validity time.Duration) *TimeEncoder {
	if source == nil {
		source = time.Now
	}

	return &TimeEncoder{
		validity: validity,
		now:      source,
	}
}

func (t *TimeEncoder) Encode(data []byte) ([]byte, error) {
	now := t.now().Unix()

	timedata := make([]byte, binary.MaxVarintLen64)
	written := binary.PutVarint(timedata, now)
	return append(timedata[:written], data...), nil
}

var ExpiredError = fmt.Errorf("signature expired")

func (t *TimeEncoder) Decode(data []byte) ([]byte, error) {
	issued, parsed := binary.Varint(data)
	if parsed <= 0 {
		return nil, fmt.Errorf("invalid timestamp in buffer")
	}

	if issued <= 0 || time.Unix(issued, 0).Add(t.validity).Before(t.now()) {
		return nil, ExpiredError
	}

	return data[parsed:], nil
}

type TypeEncoder struct {
	be BinaryEncoder
}

func NewTypeEncoder(be BinaryEncoder) *TypeEncoder {
	return &TypeEncoder{
		be: be,
	}
}

func (t *TypeEncoder) Encode(data interface{}) ([]byte, error) {
	buffer := bytes.Buffer{}
	enc := gob.NewEncoder(&buffer)
	if err := enc.Encode(data); err != nil {
		return nil, err
	}
	return t.be.Encode(buffer.Bytes())
}

func (t *TypeEncoder) Decode(data []byte, output interface{}) error {
	data, err := t.be.Decode(data)
	if err != nil {
		return err
	}

	enc := gob.NewDecoder(bytes.NewReader(data))
	if err := enc.Decode(output); err != nil {
		return err
	}
	return nil
}

type SymmetricEncoder struct {
	rng *rand.Rand

	key    []byte
	cipher cipher.AEAD
}

type SymmetricSetter func(*SymmetricEncoder) error

func UseSymmetricKey(key []byte) SymmetricSetter {
	return func(be *SymmetricEncoder) error {
		be.key = key
		return nil
	}
}

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

func (t *SymmetricEncoder) Decode(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < t.cipher.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short to contain nonce")
	}

	nonce := ciphertext[:t.cipher.NonceSize()]
	ciphertext = ciphertext[t.cipher.NonceSize():]

	plaintext, err := t.cipher.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}

type Base64Encoder struct {
	enc *base64.Encoding
}

func NewBase64UrlEncoder() *Base64Encoder {
	return &Base64Encoder{
		enc: base64.RawURLEncoding,
	}
}

func (e *Base64Encoder) Encode(data []byte) ([]byte, error) {
	dst := make([]byte, e.enc.EncodedLen(len(data)))
	e.enc.Encode(dst, data)
	return dst, nil
}
func (e *Base64Encoder) Decode(data []byte) ([]byte, error) {
	dst := make([]byte, e.enc.DecodedLen(len(data)))
	_, err := e.enc.Decode(dst, data)
	if err != nil {
		return nil, err
	}
	return dst, nil
}

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

func (t *SigningEncoder) Decode(value []byte) ([]byte, error) {
	if t.verifying == nil {
		return nil, fmt.Errorf("a verifying key must be supplied to decode data")
	}
	data, ok := sign.Open(nil, value, t.verifying.ToBytes())
	if !ok {
		return nil, fmt.Errorf("signature did not match")
	}
	return data, nil
}
