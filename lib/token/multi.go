package token

import (
	"context"
	"encoding/binary"
	"fmt"
	"github.com/enfabrica/enkit/lib/multierror"
	"math"
	"math/rand"
)

// CryptoFactory is a function capable of creating a new BinaryEncoder.
//
// If the supplied key parameter is nil, then a random key should be generated.
// Returns a BinaryEncoder - used to perform encryption and decryption - the
// key configured for the encoder (either supplied as a parameter to the function,
// or generated randomly). In case of error, returns an error.
//
// Look at SymmetricCreator for an example. More details in the definition of
// MultiKeyCryptoEncoder.
type CryptoFactory func(rng *rand.Rand, key []byte) (BinaryEncoder, []byte, error)

// Allocator is a function capable of creating a buffer to encrypt the data in.
//
// This can be used to either store the data in a specific place (eg, mmap) or
// to avoid multiple allocations in the course of filling the buffer.
type Allocator func([]byte, BinaryEncoder, []BinaryEncoder) []byte

// MultiKeyCryptoEncoder provides an encoder capable of encrypting data with
// multiple keys so it can be decrypted by any one of those keys.
//
// In literature, this is referred to as a Multi-Recipient Encryption Scheme [1],
// that can be used to implement arbitrary naive/simple hybrid ciphers [2].
//
// Just like GPG or other puplar software, It works by:
//
// 1) Generating a random key and encrypting data with this random key.
//    This is the Data Encryption Key, or DEK.
//
// 2) Encrypting this random key multiple times, once per recipient.
//    Each recipient provides a Key Encryption Key, or KEK.
//
// In the implementation here:
//
// - data is encrypted with an arbitrary BinaryEncoder, which is instantiated
//   through a CryptFactory, in charge of generating the key and initializing
//   the cipher to use. CyrptoFactory generates the DEK.
//
// - the DEK is encrypted through one or more keyholders. Keyholders are just
//   BinaryEncoders, symmetric or asymmetric, capable of encrypting the DEK
//   with their own KEK.
//
// The use of a MultiKeyCryptoEncoder is very very simple. See the test for
// more examples, but a basic use to encrypt data can look like:
//
//     keyholder1, err := NewAsymmetricEncoder(rng, UsePublicKey(recipient1))
//     ...
//     keyholder2, err := NewAsymmetricEncoder(rng, UsePublickKey(recipient2))
//     ...
//
//     mke, err := NewMultiKeyCryptoEncoder(
//                    rng, SymmetricCreator, WithKeyHolder(keyholder1, keyholder2))
//     ...
//     encoded, err := mke.Encode(data)
//
// While to decrypt the data, a single recipient could use something like:
//
//     mke, err := NewMultiKeyCryptoEncoder(
//                    rng, SymmetricCreator, WithKeyHolder(keyholder1))
//     ...
//     _, decoded, err := mke.Decode(context.Background(), encoded)
//
// Now:
//  - SymmetricCreator generates a random 256 bit key, and configures
//    an AES256-GCM cipher to encrypt/decrypt the data. You can create your
//    own factory to use any other algorithm.
//
//  - Keyholders are just other BinaryEncoders. You can use an AsymmetricEncoder,
//    a Symmetric one, mix them, or even just store the DEK in cleartext if you
//    really want to.
//
//  - MultiKeyCryptoEncoder is really agnostic to the encoder returned by
//    the CryptoFactory, or used as keyholder.
//
//  - Each call to Encode() results in a new random key (and random nonces)
//    being computed, and in all keyholders being invoked in turn to encrypt
//    that key and store the result as part of the Encode()d message.
//
//  - If you use a MultiKeyCryptoEncoder to encrypt the data, you MUST use a
//    MultiKeyCryptoEncoder to decrypt it.
//
//  - A MultiKeyCryptoEncoder will be able to decrypt the message as long as it
//    has at least one keyholder capable of decrypting one of the encrypted keys.
//
//  - Authenticated encryption for all (keyholder and data encryption) is
//    strongly recommended.
//
//    The MultiKeyCryptoEncoder stores very little metadata alongside each key
//    (just the key length). When decrypting, it will try each key in turn
//    until it finds one that (a) can be decrypted without errors, and (b) can
//    decrypt the entire message without errors.
//
//    If neither the keyholder nor the data encryption uses authenticated
//    encryption (or is chained with NewChainedEncoder with some form of
//    MAC/hashing/checksumming), it is likely that a Decode() will result in
//    garbage, as the operation will succeed even in the presence of invalid
//    keys (same that would happen with the wrong key and a non-authenticated
//    scheme).
//
//  - The data returned by a MultiKeyCryptoEncoder is neither signed nor
//    authenticated.  A receiver or MITM could modify the data and make
//    undetectable changes to the layout by, for example, removing or adding
//    keys, or corrupting the framing that indicates the lenght of each
//    stored copy of the key.
//
//    If this is undesireable, you can chain the encoder with another signing
//    or encrypting encoder. But assuming both KEK and DEK are used with an
//    authenticated encryption scheme risk should be minimal if any (extra
//    keys will be rejected, corruption in key or data will be detected).
//
// [1]: https://www.cc.gatech.edu/~aboldyre/papers/bbks.pdf
//      https://www.cc.gatech.edu/~aboldyre/papers/bbks.pdf
// [2]: https://en.wikipedia.org/wiki/Hybrid_cryptosystem
type MultiKeyCryptoEncoder struct {
	rng       *rand.Rand
	allocate  Allocator
	creator   CryptoFactory
	keyholder []BinaryEncoder
}

type MultiKeyCryptoSetter func(*MultiKeyCryptoEncoder) error

func WithKeyHolder(be ...BinaryEncoder) MultiKeyCryptoSetter {
	return func(mke *MultiKeyCryptoEncoder) error {
		mke.keyholder = append(mke.keyholder, be...)
		return nil
	}
}

func WithAllocator(allocator Allocator) MultiKeyCryptoSetter {
	return func(mke *MultiKeyCryptoEncoder) error {
		mke.allocate = allocator
		return nil
	}
}

var DefaultAllocator Allocator = func(buffer []byte, cipher BinaryEncoder, keyciphers []BinaryEncoder) []byte {
	// A 256 bit key is 32 bytes, same for a 256 bits nonce.
	// An authenticated encryption scheme can add X more bytes (16? 32?) of some form of MAC.
	//
	// Per key, and per message, we have to store a varint of up to ~8 bytes.
	// Approximate Key + Nonce + MAC bytes + varint per key stored + Nonce + MAC,
	// round both numbers to the closest multiple of 32.
	return make([]byte, 0, len(buffer)+64+96*len(keyciphers))
}

// NewMultiKeyCryptoEncoder creates a new MultiKeyCryptoEncoder.
//
// creator is a function capable of generating a random key and a BinaryEncoder to use
// to encode the data.
//
// With settters, at least one keyholder must be specified.
func NewMultiKeyCryptoEncoder(rng *rand.Rand, creator CryptoFactory, setters ...MultiKeyCryptoSetter) (*MultiKeyCryptoEncoder, error) {
	mke := &MultiKeyCryptoEncoder{
		rng:      rng,
		allocate: DefaultAllocator,
		creator:  creator,
	}

	for _, setter := range setters {
		if err := setter(mke); err != nil {
			return nil, err
		}
	}

	if len(mke.keyholder) <= 0 {
		return nil, fmt.Errorf("no key holder specified - at least one keyholder is required")
	}
	return mke, nil
}

// Encode encrypts the data so that it can be decrypted by any one keyholder.
//
// Encode invokes the configured CryptoFactory to generate a random key and a BinaryEncoder
// to encrypt the data. It then invokes each keyholder in turn to encrypt this random key
// alongside the encrypted message.
//
// The returned byte array has the format:
//   [varint: length of ciphertext][ciphertext]
//   [varint: length of key encrypted with keyholder[0]][key encrypted with keyholder[0]]
//   [varint: length of key encrypted with keyholder[1]][key encrypted with keyholder[1]]
//   [...]
func (mke *MultiKeyCryptoEncoder) Encode(data []byte) ([]byte, error) {
	sc, key, err := mke.creator(mke.rng, nil)
	if err != nil {
		return nil, err
	}

	result := mke.allocate(data, sc, mke.keyholder)
	lenbuff := make([]byte, binary.MaxVarintLen64)
	add := func(data []byte) {
		n := binary.PutUvarint(lenbuff, uint64(len(data)))
		result = append(result, lenbuff[:n]...)
		result = append(result, data...)
	}

	cipherdata, err := sc.Encode(data)
	if err != nil {
		return nil, err
	}

	add(cipherdata)
	for _, kh := range mke.keyholder {
		ek, err := kh.Encode(key)
		if err != nil {
			return nil, err
		}
		add(ek)
	}

	return result, nil
}

// Decode decrypts a message created with Encode.
//
// Given that the MultiKeyCryptoEncoder used to Decode the message is expected
// to be initialized with a single key or a small subset of the keys used to
// encode the message, Decode will simply try to decode each key with each
// keyholder configured.
//
// If it finds one key that can be decoded successfully by one of its keyholders,
// and this key can decrypt the data without errors, Decode() will return success
// with the result of the operation.
func (mke *MultiKeyCryptoEncoder) Decode(ctx context.Context, buffer []byte) (context.Context, []byte, error) {
	cipherlen, uintlen := binary.Uvarint(buffer)
	if uintlen <= 0 || cipherlen > math.MaxInt32 || cipherlen <= 0 || (uintlen+int(cipherlen)) > len(buffer) {
		return ctx, nil, fmt.Errorf("ciphertext: buffer is too small, or not encoded correctly - %d %d", cipherlen, uintlen)
	}

	ciphertext := buffer[uintlen : uintlen+int(cipherlen)]
	keybuffer := buffer[uintlen+int(cipherlen):]
	var errors []error
	for keyid := 0; len(keybuffer) > 0; keyid += 1 {
		keylen, uintlen := binary.Uvarint(keybuffer)
		if uintlen <= 0 || keylen > math.MaxInt32 || keylen <= 0 || (uintlen+int(keylen)) > len(keybuffer) {
			return ctx, nil, fmt.Errorf("key[%d]: buffer is too small, or not encoded correctly", keyid)
		}
		currentkey := keybuffer[uintlen : uintlen+int(keylen)]
		keybuffer = keybuffer[uintlen+int(keylen):]

		var decodedkey []byte
		var cleartext []byte
		var err error
		for cx, kh := range mke.keyholder {
			ctx, decodedkey, err = kh.Decode(ctx, currentkey)
			if err != nil {
				errors = append(errors, fmt.Errorf("key[%d], cipher[%d]: key cannot be decoded - %w", keyid, cx, err))
				continue
			}

			sc, _, err := mke.creator(mke.rng, decodedkey)
			if err != nil {
				errors = append(errors, fmt.Errorf("key[%d], cipher[%d]: key cannot be used - %w", keyid, cx, err))
				continue
			}

			ctx, cleartext, err = sc.Decode(ctx, ciphertext)
			if err != nil {
				errors = append(errors, fmt.Errorf("key[%d], cipher[%d]: key cannot decode ciphertext - %w", keyid, cx, err))
				continue
			}

			return ctx, cleartext, nil
		}

	}
	return ctx, nil, multierror.NewOr(errors, fmt.Errorf("no valid key could be found"))
}
