package kcerts

import (
	"crypto"
	"errors"
	"golang.org/x/crypto/ssh"
	"io"
)

type customSigner struct {
	algorithm string
	signer    ssh.AlgorithmSigner
}

func (s *customSigner) PublicKey() ssh.PublicKey {
	return s.signer.PublicKey()
}

func (s *customSigner) Sign(rand io.Reader, data []byte) (*ssh.Signature, error) {
	return s.signer.SignWithAlgorithm(rand, data, s.algorithm)
}

func NewSSHSigner(signer crypto.Signer, algo string) (ssh.Signer, error) {
	sshSigner, err := ssh.NewSignerFromSigner(signer)
	if err != nil {
		return nil, err
	}
	algorithmSigner, ok := sshSigner.(ssh.AlgorithmSigner)
	if !ok {
		return nil, errors.New("unable to cast to ssh.AlgorithmSigner")
	}
	s := customSigner{
		signer:    algorithmSigner,
		algorithm: algo,
	}
	return &s, nil
}
