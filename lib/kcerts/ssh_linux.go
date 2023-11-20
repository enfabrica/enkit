// +build linux
package kcerts

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"net"
	"time"
)

func DialTimeout(a SSHAgent) (net.Conn, error) {
	return net.DialTimeout("unix", a.State.Socket, a.timeout)
}

func AddKey(conn net.Conn, a SSHAgent, privateKey PrivateKey, publicKey ssh.PublicKey) error {
	cert, ok := publicKey.(*ssh.Certificate)
	if !ok {
		return fmt.Errorf("public key is not a valid ssh certificate")
	}
	ttl := SSHCertRemainingTTL(cert)
	if ttl == InValidCertTimeDuration {
		return fmt.Errorf("certificate is already expired or invalid, not adding")
	}

	conn.SetDeadline(time.Now().Add(a.timeout))
	return agent.NewClient(conn).Add(agent.AddedKey{
		PrivateKey:   privateKey.Raw(),
		Certificate:  cert,
		LifetimeSecs: uint32(ttl.Seconds()),
	})
}
