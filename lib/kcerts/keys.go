package kcerts

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"golang.org/x/crypto/ssh"
)

func NewSigner(key PrivateKey) (ssh.Signer, error) {
	res, err := NewSSHSigner(key.Signer(), key.SigningAlgo())
	if err != nil {
		return nil, err
	}
	return res, nil
}

type PrivateKey interface {
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
	res, err := x509.MarshalPKCS8PrivateKey(e.rawKey)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: res}), nil
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
type SSHKeyGenerator func() (ssh.PublicKey, PrivateKey, error)

var (
	_               SSHKeyGenerator = GenerateRSA
	_               SSHKeyGenerator = GenerateED25519
	GenerateDefault SSHKeyGenerator = GenerateED25519
)

func GenerateRSA() (ssh.PublicKey, PrivateKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, err
	}
	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, nil, err
	}
	return publicKey, rsaProvider{p: privateKey}, err
}

func GenerateED25519() (ssh.PublicKey, PrivateKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return nil, nil, err
	}
	return sshPub, ed25519Provider{rawKey: priv}, err
}

func FromEC25519(key ed25519.PrivateKey) PrivateKey {
	return ed25519Provider{
		rawKey: key,
	}
}

func FromRSA(key *rsa.PrivateKey) PrivateKey {
	return rsaProvider{
		p: key,
	}
}
