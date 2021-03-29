package kcerts

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"github.com/mitchellh/go-homedir"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const CAPrefix = "@cert-authority"
const SSHDir = ".ssh"
const KnownHosts = "known_hosts"

// AddSSHCAToClient adds a public key to the $HOME/.ssh/known_hosts in the ssh-cert x509.1 format.
// For each entry, it adds an additional line and does not concatenate
func AddSSHCAToClient(publicKey ssh.PublicKey, hosts []string) error {
	hDir, err := homedir.Dir()
	if err != nil {
		return err
	}
	sshDir := filepath.Join(hDir, SSHDir)
	if _, err := os.Stat(sshDir); os.IsNotExist(err) {
		return fmt.Errorf("ssh directory %s does not exist, please create it", hDir+SSHDir)
	}
	knownHosts := filepath.Join(sshDir, KnownHosts)
	if _, err := os.Stat(knownHosts); os.IsNotExist(err) {
		return fmt.Errorf("ssh authorized hosts file %s does not exist, please create it", knownHosts)
	}
	caPublic := string(ssh.MarshalAuthorizedKey(publicKey))
	existingKnownHostsContent, err := ioutil.ReadFile(knownHosts)
	if err != nil {
		return err
	}
	knownHostsFile, err := os.OpenFile(knownHosts, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer knownHostsFile.Close()
	for _, dns := range hosts {
		publicFormat := fmt.Sprintf("%s %s %s", CAPrefix, dns, caPublic)
		if strings.Contains(string(existingKnownHostsContent), publicFormat) {
			continue
		}
		_, err = knownHostsFile.WriteString(publicFormat)
		if err != nil {
			return fmt.Errorf("could not add key %s to known_hosts file: %w", err)
		}
	}
	return nil
}

// StartSSHAgent Will start the ssh agent in the interactive terminal if it isn't present already as an environment variable
// Currently only outputs the env and does not persist it across terminals
func StartSSHAgent() error {
	if os.Getenv("SSH_AUTH_SOCK") != "" {
		return nil
	}
	cmd := exec.Command("ssh-agent", "-s")
	out, err := cmd.Output()
	if err != nil {
		return err
	}
	reader := bufio.NewScanner(bytes.NewReader(out))
	for reader.Scan() {
		if strings.Contains(reader.Text(), "SSH_AUTH_SOCK") {
			afterSockString := strings.SplitN(reader.Text(), "SSH_AUTH_SOCK=", 2)
			socketPath := strings.Split(afterSockString[1], ";")
			os.Setenv("SSH_AUTH_SOCK", strings.TrimSpace(socketPath[0]))
			fmt.Println("set SSH_AUTH_SOCK to", os.Getenv("SSH_AUTH_SOCK"))
		}
	}
	return err
}

// GenerateUserSSHCert will sign and return credentials based on the CA signer and given parameters
// to generate a user cert, certType must be 1, and host certs ust have certType 2
func GenerateUserSSHCert(ca ssh.Signer, certType uint32, principals []string, ttl time.Duration) (*rsa.PrivateKey, *ssh.Certificate, error) {
	priv, pub, err := makeKeys()
	if err != nil {
		return priv, nil, err
	}
	from := time.Now().UTC()
	to := time.Now().UTC().Add(ttl * time.Hour)
	cert := &ssh.Certificate{
		CertType:        certType,
		Key:             pub,
		ValidAfter:      uint64(from.Unix()),
		ValidBefore:     uint64(to.Unix()),
		ValidPrincipals: principals,
		Permissions:     ssh.Permissions{},
	}
	if err := cert.SignCert(rand.Reader, ca); err != nil {
		return nil, nil, err
	}
	return priv, cert, nil
}

func makeKeys() (*rsa.PrivateKey, ssh.PublicKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, nil, err
	}
	return privateKey, publicKey, err
}
