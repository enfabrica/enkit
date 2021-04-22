package kcerts

import (
"crypto"
"errors"
"golang.org/x/crypto/ssh"
"io"
)

type sha256Signer struct {
	algorithm string
	signer    ssh.AlgorithmSigner
}

func (s *sha256Signer) PublicKey() ssh.PublicKey {
	return s.signer.PublicKey()
}

func (s *sha256Signer) Sign(rand io.Reader, data []byte) (*ssh.Signature, error) {
	return s.signer.SignWithAlgorithm(rand, data, s.algorithm)
}

func NewSha256SignerFromSigner(signer crypto.Signer) (ssh.Signer, error) {
	sshSigner, err := ssh.NewSignerFromSigner(signer)
	if err != nil {
		return nil, err
	}
	algorithmSigner, ok := sshSigner.(ssh.AlgorithmSigner)
	if !ok {
		return nil, errors.New("unable to cast to ssh.AlgorithmSigner")
	}
	s := sha256Signer{
		signer:    algorithmSigner,
		algorithm:  ssh.KeyAlgoED25519,
	}
	return &s, nil
}
