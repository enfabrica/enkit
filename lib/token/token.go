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
	"context"
	"encoding/base64"
	"github.com/enfabrica/enkit/lib/config/marshal"
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

type TypeEncoder struct {
	be BinaryEncoder
	ma marshal.Marshaller
}

type TypeEncoderSetter func(*TypeEncoder)

// WithMarshaller selects a specific marshaller to use with NewTypeEncoder.
//
// If none is specified, by default NewTypeEncoder will use a gob encoder.
// Note that different marshaller may impose different constraints.
func WithMarshaller(ma marshal.Marshaller) TypeEncoderSetter {
	return func(te *TypeEncoder) {
		te.ma = ma
	}
}

func NewTypeEncoder(be BinaryEncoder, setter ...TypeEncoderSetter) *TypeEncoder {
	te := &TypeEncoder{
		be: be,
		ma: marshal.Gob,
	}

	for _, set := range setter {
		set(te)
	}
	return te
}

func (t *TypeEncoder) Encode(data interface{}) ([]byte, error) {
	buffer, err := t.ma.Marshal(data)
	if err != nil {
		return nil, err
	}
	return t.be.Encode(buffer)
}

func (t *TypeEncoder) Decode(ctx context.Context, data []byte, output interface{}) (context.Context, error) {
	ctx, data, derr := t.be.Decode(ctx, data)
	if data == nil && derr != nil {
		return ctx, derr
	}

	nerr := t.ma.Unmarshal(data, output)

	err := derr
	if err == nil {
		err = nerr
	}
	return ctx, err
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
