// Package token provides primitives to create and decode cryptographic tokens.
//
// The library is built around the concept of Encoders: objects capable of turning
// a byte array into another, by, for example, adding a cryptographic signature
// created with an asymmetric key, encrypting the data, adding an expiry time, or
// by chaining multiple encoders together.
//
// Additionally, the library provides a few higher level adapters that allow to
// serialize golang structs into an array of bytes, or to turn an array of bytes
// into a string.
//
// For example, by using something like:
//
//     be, err := token.NewSymmetricEncoder(...)
//     if err ...
//     
//     encoder := token.NewTypeEncoder(token.NewChainedEncoder(
//         token.NewTimeEncoder(nil, time.Second * 10), be, token.NewBase64URLEncoder()) 
//
// you will get an encoder that when used like:
//
//      uData := struct {
//        Username, Lang string
//      }{"myname", "english"}
//
//      b64string, err := encoder.Encode(uData)
//
// will convert a struct into a byte array, add the time the serialization happened,
// encrypt all with a symmetric key, and then convert to base64.
//
// On Decode(), the original array will be returned after applying all the necessary
// transformations and verifications. For example, Decode() will error out if the data
// is older than 10 seconds, the maximum lifetime supplied to NewTimeEncoder.
// 
package token

import (
	"bytes"
	"context"
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

// Used internally to define keys exported via context.
type contextKey string

// BinaryEncoders convert an array of bytes into another by applying binary
// transformations.
//
// For example: they can encrypt the data, compress it, sign it, augment it
// with metadata (like an expiration time), and so on.
type BinaryEncoder interface {
	// Encode will transform the input array of bytes into the returned one.
	Encode([]byte) ([]byte, error)

	// Decode will return the original array of bytes after decoding it.
	//
	// The context can be used to access additional metadata.
	// See examples below.
	Decode(context.Context, []byte) (context.Context, []byte, error)
}

// ChainedEncoder is a set of BinaryEncoders to be applied in sequence.
//
// This allows, for example, to add additional signatures to data after
// encrypting it, or to add an expiration time.
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

func (ce *ChainedEncoder) Decode(ctx context.Context, data []byte) (context.Context, []byte, error) {
	encs := ([]BinaryEncoder)(*ce)
	var first error
	for ix := range encs {
		enc := encs[len(encs)-ix-1]

		var err error
		ctx, data, err = enc.Decode(ctx, data)
		if err != nil {
			if first == nil {
				first = err
			}
			if data == nil {
				break
			}
		}
	}
	return ctx, data, first
}

// StringEncoders convert an array of bytes into a string safe for specific applications.
//
// For example: mime64 encoding, url encoding, hex, ...
type StringEncoder interface {
	Encode([]byte) (string, error)
	Decode(context.Context, string) (context.Context, []byte, error)
}

// TimeSource is a function that returns the current time.
type TimeSource func() time.Time

// TimeEncoder is an encoder that saves the time the data was encoded.
//
// On Decode, it checks with the supplied validity and time source, and
// fails validation if the data is considered expired.
//
// If data is expired is determined solely by the consumer of the data,
// based on the time the data was created.
//
// Expiry information is not encoded in the resulting byte array.
type TimeEncoder struct {
	validity time.Duration
	now      TimeSource
}

// NewTimeEncoder creates a new TimeEncoder.
//
// source is a TimeSource to read the time from.
// validity is used on decode together with the issued time carried with
// the data to determine if the data is to be considered expired or not.
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

// ExpiredError is returned if the data is considered expired.
var ExpiredError = fmt.Errorf("signature expired")

// IssuedTimeKey allows to access the time encoded by TimeEncoder.Encode.
//
// During Deocde() the context supplied is annotated with the time extracted
// while decoding the data.
//
// Example:
//   te := NewTimeEncoder(...)
//   ...
//   ctx, data, err := te.Decode(context.Background(), original)
//   ...
//   etime, ok := ctx.Value(token.IssuedTimeKey).(time.Time)
//   if !ok {
//     ...
//   }
var IssuedTimeKey = contextKey("issued")

// MaxTimeKey allows to access the maximum validity of the data.
//
// MaxTimeKey can be accessed and used just like explained for IssuedTimeKey.
var MaxTimeKey = contextKey("max")

// Decode decodes TimeEncoder encoded data.
//
// It returns ExpiredError if the data was issued before the
// validity time supplied to NewTimeEncoder.
// It returns a generic error if the data is considered corrupted
// or invalid for any other reason.
//
// Decode always tries to return as much data as possible, together
// with IssuedTime and MaxTime information in the context, even
// if the data is expired.
// This allows, for example, to write code to override/ignore the ExpiredError,
// or to print user friendly messages indicating when the data was expired.
func (t *TimeEncoder) Decode(ctx context.Context, data []byte) (context.Context, []byte, error) {
	issued, parsed := binary.Varint(data)
	if parsed <= 0 {
		return ctx, nil, fmt.Errorf("invalid timestamp in buffer")
	}

	itime := time.Unix(issued, 0)
	ctx = context.WithValue(ctx, IssuedTimeKey, itime)

	max := itime.Add(t.validity)
	ctx = context.WithValue(ctx, MaxTimeKey, max)

	if issued <= 0 || max.Before(t.now()) {
		return ctx, data[parsed:], ExpiredError
	}
	return ctx, data[parsed:], nil
}

// ExpireEncoder is an encoder that saves the time the data expires.
//
// On Decode, it checks with the supplied time source, and fails validation if
// the data is considered expired.
//
// This means that the Encode()r of the data is in control of when the
// clients using Decode() will consider it expired, as they will generally
// enforce the stored expiry time.
//
// Expiry information is encoded in the token by whoever created the data.
type ExpireEncoder struct {
	validity time.Duration
	now      TimeSource
}

// NewExpireEncoder creates a new ExpireEncoder.
//
// source is a source of time, TimeSource.
// validity is the dessired lifetime of the data. It is used during encode to
// store a desired expire time alongisde the data.
func NewExpireEncoder(source TimeSource, validity time.Duration) *ExpireEncoder {
	if source == nil {
		source = time.Now
	}

	return &ExpireEncoder{
		validity: validity,
		now:      source,
	}
}

func (t *ExpireEncoder) Encode(data []byte) ([]byte, error) {
	expireson := t.now().Add(t.validity).Unix()

	timedata := make([]byte, binary.MaxVarintLen64)
	written := binary.PutVarint(timedata, expireson)
	return append(timedata[:written], data...), nil
}

// ExpiresTimeKey allows to access the time the data is expected to expire.
//
// It can be accessed and used just like explained for IssuedTimeKey.
var ExpiresTimeKey = contextKey("expire")

// Decode decodes ExpireEncoder encoded data.
//
// It returns ExpiredError if the time supplied by the passed TimeSource is
// past the ExpiresTime carried alongside the data.
// It returns a generic error if the data is considered corrupted or invalid
// for any other reason.
//
// Decode always tries to return as much data as possible, together with
// ExpiresTime information in the context, even if the data is expired.
//
// This allows, for example, to write code to override/ignore the ExpiredError,
// or to print user friendly messages indicating when the data was expired.
func (t *ExpireEncoder) Decode(ctx context.Context, data []byte) (context.Context, []byte, error) {
	expires, parsed := binary.Varint(data)
	if parsed <= 0 {
		return ctx, nil, fmt.Errorf("invalid timestamp in buffer")
	}

	expirest := time.Unix(expires, 0)
	ctx = context.WithValue(ctx, ExpiresTimeKey, expirest)

	if expires <= 0 || expirest.Before(t.now()) {
		return ctx, data[parsed:], ExpiredError
	}
	return ctx, data[parsed:], nil
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

func (t *TypeEncoder) Decode(ctx context.Context, data []byte, output interface{}) (context.Context, error) {
	ctx, data, derr := t.be.Decode(ctx, data)
	if data == nil && derr != nil {
		return ctx, derr
	}

	enc := gob.NewDecoder(bytes.NewReader(data))
	nerr := enc.Decode(output)

	err := derr
	if derr == nil {
		err = nerr
	}
	return ctx, err
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
func (e *Base64Encoder) Decode(ctx context.Context, data []byte) (context.Context, []byte, error) {
	dst := make([]byte, e.enc.DecodedLen(len(data)))
	_, err := e.enc.Decode(dst, data)
	if err != nil {
		return ctx, nil, err
	}
	return ctx, dst, nil
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
