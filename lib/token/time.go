package token

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"
)

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
