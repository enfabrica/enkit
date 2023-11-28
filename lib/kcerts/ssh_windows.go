// +build windows
package kcerts

import (
	"fmt"
	"github.com/Microsoft/go-winio"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"net"
	"time"
)

// When talking to the SSH agent on linux machines, use unix sockets
// while use named pipes for windows machines.
// https://learn.microsoft.com/en-us/windows/win32/ipc/named-pipes
func DialTimeout(a SSHAgent) (net.Conn, error) {
	return winio.DialPipe(a.State.Socket, &a.timeout)
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
	// BUG(INFRA-8631): Do not add LifetimeSecs to AddedKey because the lifetime of the cert
	// is already embedded in the cert returned by the auth server. Adding LifetimeSecs
	// again will cause the ssh-agent on Windows to experience an impersonation token error.
	// This failure causes the ssh-agent on Windows to not respond back to enkit via RPC
	// which triggers an obscure EOF error.
	return agent.NewClient(conn).Add(agent.AddedKey{
		PrivateKey:  privateKey.Raw(),
		Certificate: cert,
	})
}
