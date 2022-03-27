package kcerts

import (
	"golang.org/x/crypto/ssh"
)

type CertMod func(certificate *ssh.Certificate) *ssh.Certificate

var (
	NoOp CertMod = func(certificate *ssh.Certificate) *ssh.Certificate {
		return certificate
	}

	AddExtensionMod = func(key, value string) CertMod {
		return func(certificate *ssh.Certificate) *ssh.Certificate {
			certificate.Extensions[key] = value
			return certificate
		}
	}
)
