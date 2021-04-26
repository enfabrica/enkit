package kcerts

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"golang.org/x/crypto/ssh"
	"strings"
)

type PrivateKey struct {
	Key PrivateKeyProvider
}

func (e PrivateKey) Signer() (ssh.Signer, error) {
	res, err := NewSSHSigner(e.Key.Signer(), e.Key.SigningAlgo())
	if err != nil {
		return nil, err
	}
	return res, nil
}

type PrivateKeyProvider interface {
	Signer() crypto.Signer
	SigningAlgo() string
	Raw() interface{}
	SSHPemEncode() ([]byte, error)
}

// ed25519Provider wraps utility ssh functions around for PrivateKey. This is due to a featureset missing in the standard
// ssh library
type ed25519Provider struct {
	rawKey ed25519.PrivateKey
}

func (e ed25519Provider) Signer() crypto.Signer {
	return e.rawKey
}

func (e ed25519Provider) SSHPemEncode() ([]byte, error) {
	res, err := OpenSSHEncode21559PrivateKey(e.rawKey)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (e ed25519Provider) Raw() interface{} {
	return e.rawKey
}

func (e ed25519Provider) SigningAlgo() string {
	return ssh.KeyAlgoED25519
}

// rsaProvider wraps utility ssh functions around for PrivateKey. This is due to a featureset missing in the standard
// ssh library. When signing does not implement ssh-rsa, but rather ssh-rsa-512 which is supported by sshd(8)
type rsaProvider struct {
	p *rsa.PrivateKey
}

func (r rsaProvider) Signer() crypto.Signer {
	return r.p
}

func (r rsaProvider) SigningAlgo() string {
	return ssh.SigAlgoRSASHA2512
}

func (r rsaProvider) Raw() interface{} {
	return r.p
}

func (r rsaProvider) SSHPemEncode() ([]byte, error) {
	b := x509.MarshalPKCS1PrivateKey(r.p)
	return pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: b}), nil
}

// KeyGenerator is A function capable of generating a key pair.
//
// The function is expected to return a Public Key, a Private Key,
// and an error.
//
// Only keys supported by x/crypto/ssh.NewPublicKey are supported.
type SSHKeyGenerator func() (*PrivateKey, ssh.PublicKey, error)

// MakeKeys uses the speficied key generator to create a pair of ssh keys.
//
// The first return value is a marshalled version of the public key,
// a binary blob suitable for transmission.
// The second return value is the private key in the original format.
// This is generally directly usable with functions like agent.Add.
func MakeKeys(generator SSHKeyGenerator) (*PrivateKey, ssh.PublicKey, error) {
	private, public, err := generator()
	if err != nil {
		return nil, nil, err
	}
	return private, public, nil
}

func GenerateRSA() (*PrivateKey, ssh.PublicKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, err
	}
	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, nil, err
	}
	return &PrivateKey{Key: rsaProvider{p: privateKey}}, publicKey, err
}

func GenerateED25519() (*PrivateKey, ssh.PublicKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return nil, nil, err
	}
	return &PrivateKey{Key: ed25519Provider{rawKey: priv}}, sshPub, err
}

var GenerateDefault = GenerateED25519

// SelectGenerator returns a different generator based on the specified string.
//
// Currently it accepts "rsa", or "ed25519".
func SelectGenerator(name string) SSHKeyGenerator {
	name = strings.ToLower(name)
	if name == "rsa" {
		return GenerateRSA
	}
	if name == "ed25519" {
		return GenerateED25519
	}
	return nil
}

func FromEC25519(key ed25519.PrivateKey) *PrivateKey {
	return &PrivateKey{
		Key: ed25519Provider{
			rawKey: key,
		},
	}
}

func FromRSA(key *rsa.PrivateKey) *PrivateKey {
	return &PrivateKey{
		Key: rsaProvider{
			p: key,
		},
	}
}
