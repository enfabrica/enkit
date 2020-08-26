package common

import (
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"strings"
)

const KeyLength = 32
const NonceLength = 24

type Key [KeyLength]byte

func (k *Key) ToByte() *[KeyLength]byte {
	return (*[KeyLength]byte)(k)
}

type Nonce [NonceLength]byte

func (n *Nonce) ToByte() *[NonceLength]byte {
	return (*[NonceLength]byte)(n)
}

func NonceFromSlice(nonce []byte) (*Nonce, error) {
	parsedNonce := Nonce{}
	if copy(parsedNonce[:], nonce) != NonceLength || len(nonce) > NonceLength {
		return nil, fmt.Errorf("invalid nonce length - must be %d bytes", NonceLength)
	}
	return &parsedNonce, nil
}

func KeyFromSlice(key []byte) (*Key, error) {
	parsedKey := Key{}
	if copy(parsedKey[:], key) != KeyLength || len(key) > KeyLength {
		return nil, fmt.Errorf("invalid key length - must be %d bytes", KeyLength)
	}
	return &parsedKey, nil
}

func KeyFromString(key string) (*Key, error) {
	return KeyFromSlice([]byte(key))
}

func KeyFromHex(key string) (*Key, error) {
	slice, err := hex.DecodeString(key)
	if err != nil {
		return nil, err
	}
	return KeyFromSlice(slice)
}

func KeyFromURL(url string) (*Key, error) {
	ix := strings.LastIndex(url, "/")
	if ix < 0 {
		return nil, fmt.Errorf("invalid URL - does not match expected format")
	}
	return KeyFromHex(url[ix+1:])
}

func init() {
	gob.Register(Key{})
}
