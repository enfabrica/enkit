package kcerts

import (
	"crypto/rand"
	stdRand "math/rand"
	"encoding/pem"
	"fmt"
	"github.com/enfabrica/enkit/lib/cache"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/mitchellh/go-homedir"
	"golang.org/x/crypto/ed25519"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	CAPrefix   = "@cert-authority"
	SSHDir     = ".ssh"
	KnownHosts = "known_hosts"
)

var (
	sockR = regexp.MustCompile("(?m)SSH_AUTH_SOCK=([^;\\n]*)")
	pidR  = regexp.MustCompile("(?m)SSH_AGENT_PID=([0-9]*)")
)

// FindSSHDir will find the users ssh directory based on $HOME. If $HOME/.ssh does not exist
// it will attempt to create it.
func FindSSHDir() (string, error) {
	hDir, err := homedir.Dir()
	if err != nil {
		return "", fmt.Errorf("could not find the home directory: %w", err)
	}
	sshDir := filepath.Join(hDir, SSHDir)
	if err := os.Mkdir(sshDir, 0700); err != nil && !os.IsExist(err) {
		return "", fmt.Errorf("could not create directory %s: %w", sshDir, err)
	}
	return sshDir, nil
}

// AddSSHCAToClient adds a public key to the $HOME/.ssh/known_hosts in the ssh-cert x509.1 format.
// For each entry, it adds an additional line and does not concatenate.
func AddSSHCAToClient(publicKey ssh.PublicKey, hosts []string, sshDir string) error {
	caPublic := string(ssh.MarshalAuthorizedKey(publicKey))
	knownHosts := filepath.Join(sshDir, KnownHosts)
	knownHostsFile, err := os.OpenFile(knownHosts, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("could not create known_hosts file: %w", err)
		}
		return err
	}
	defer knownHostsFile.Close()
	existingKnownHostsContent, err := ioutil.ReadAll(knownHostsFile)
	if err != nil {
		return fmt.Errorf("error reading %s: %w", knownHosts, err)
	}
	for _, dns := range hosts {
		// caPublic terminates with a '\n', added by ssh.MarshalAuthorizedKey
		publicFormat := fmt.Sprintf("%s %s %s", CAPrefix, dns, caPublic)
		if strings.Contains(string(existingKnownHostsContent), publicFormat) {
			continue
		}
		_, err = knownHostsFile.WriteString(publicFormat)
		if err != nil {
			return fmt.Errorf("could not add key %s to known_hosts file: %w", publicFormat, err)
		}
	}
	return nil
}

type SSHAgent struct {
	PID    int    `json:"pid"`
	Socket string `json:"sock"`
	// Close is edited in WriteToCache, is defaulted to an empty lambda
	Close func() `json:"-"`
}

func (a SSHAgent) Kill() error {
	p, err := os.FindProcess(a.PID)
	if err != nil {
		return err
	}
	return p.Kill()
}

func (a SSHAgent) Valid() bool {
	conn, err := net.Dial("unix", a.Socket)
	if err != nil {
		return false
	}
	defer conn.Close()
	return err == nil
}

// AddCertificates loads an ssh certificate into the agent.
// privateKey must be a key type accepted by the golang.org/x/ssh/agent AddedKey struct.
// At time of writing, this can be: *rsa.PrivateKey, *dsa.PrivateKey, ed25519.PrivateKey or *ecdsa.PrivateKey.
// Note that ed25519.PrivateKey should be passed by value.
func (a SSHAgent) AddCertificates(privateKey PrivateKey, publicKey ssh.PublicKey, ttl uint32) error {
	conn, err := net.Dial("unix", a.Socket)
	if err != nil {
		return err
	}
	defer conn.Close()
	cert, ok := publicKey.(*ssh.Certificate)
	if !ok {
		return fmt.Errorf("public key is not a valid ssh certificate")
	}
	agentClient := agent.NewClient(conn)
	return agentClient.Add(agent.AddedKey{
		PrivateKey:   privateKey.Raw(),
		Certificate:  cert,
		LifetimeSecs: ttl,
	})
}

func (a SSHAgent) GetEnv() []string {
	return []string{fmt.Sprintf("SSH_AUTH_SOCK=%s", a.Socket), fmt.Sprintf("SSH_AGENT_PID=%d", a.PID)}
}

// FindSSHAgent Will start the ssh agent in the interactive terminal if it isn't present already as an environment variable
// It will pull, in order: from the env, from the cache, create new.
func FindSSHAgent(store cache.Store, logger logger.Logger) (*SSHAgent, error) {
	agent := FindSSHAgentFromEnv()
	if agent != nil && agent.Valid() {
		return agent, nil
	}
	agent, err := FetchSSHAgentFromCache(store)
	if err != nil {
		logger.Warnf("%s", err)
	}
	if agent != nil && agent.Valid() {
		return agent, nil
	}
	newAgent, err := CreateNewSSHAgent()
	if err != nil {
		return nil, err
	}
	logger.Infof("%s", WriteAgentToCache(store, newAgent))
	return newAgent, nil
}

// FindSSHAgentFromEnv
func FindSSHAgentFromEnv() *SSHAgent {
	envSSHSock := os.Getenv("SSH_AUTH_SOCK")
	envSSHPID := os.Getenv("SSH_AGENT_PID")
	if envSSHSock != "" || envSSHPID != "" {
		return nil
	}
	pid, err := strconv.Atoi(envSSHPID)
	if err != nil {
		return nil
	}
	return &SSHAgent{PID: pid, Socket: envSSHSock, Close: func() {}}
}

// CreateNewSSHAgent creates a new ssh agent. Its env variables have not been added to the shell. It does not maintain
// its own connection.
func CreateNewSSHAgent() (*SSHAgent, error) {
	cmd := exec.Command("ssh-agent", "-s")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	resultSock := sockR.FindStringSubmatch(string(out))
	resultPID := pidR.FindStringSubmatch(string(out))
	if len(resultSock) != 2 || len(resultPID) != 2 {
		return nil, fmt.Errorf("not a valid pid or agent sock, %v %v", resultSock, resultPID)
	}
	// The second element is the raw value we want
	rawPID := resultPID[1]
	rawSock := resultSock[1]

	pid, err := strconv.Atoi(rawPID)
	if err != nil {
		return nil, fmt.Errorf("error processing ssh agent pid %s: %w", resultPID, err)
	}
	s := &SSHAgent{Socket: rawSock, PID: pid}
	s.Close = func() {
		_ = s.Kill()
	}
	return s, nil
}

// SignPublicKey will sign and return credentials based on the CA signer and given parameters
// to generate a user cert, certType must be 1, and host certs ust have certType 2
func SignPublicKey(p PrivateKey, certType uint32, principals []string, ttl time.Duration, pub ssh.PublicKey) (*ssh.Certificate, error) {
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
	s, err := NewSigner(p)
	if err != nil {
		return nil, err
	}
	if err := cert.SignCert(rand.Reader, s); err != nil {
		return nil, err
	}
	return cert, nil
}

func OpenSSHEncode21559PrivateKey(key ed25519.PrivateKey) ([]byte, error) {
	b := openSSHMarshalED25519PrivateKey(key)
	return pem.EncodeToMemory(&pem.Block{Type: "OPENSSH PRIVATE KEY", Bytes: b}), nil

}

// Note: copited from https://github.com/mikesmitty/edkey/blob/master/edkey.go
// TODO(adam): support passwords

//* Writes ed25519 private keys into the new OpenSSH private key format.
//I have no idea why this isn't implemented anywhere yet, you can do seemingly
//everything except write it to disk in the OpenSSH private key format. */
func openSSHMarshalED25519PrivateKey(key ed25519.PrivateKey) []byte {
	// Add our key header (followed by a null byte)
	magic := append([]byte("openssh-key-v1"), 0)

	var w struct {
		CipherName   string
		KdfName      string
		KdfOpts      string
		NumKeys      uint32
		PubKey       []byte
		PrivKeyBlock []byte
	}

	// Fill out the private key fields
	pk1 := struct {
		Check1  uint32
		Check2  uint32
		Keytype string
		Pub     []byte
		Priv    []byte
		Comment string
		Pad     []byte `ssh:"rest"`
	}{}

	// Set our check ints
	ci := stdRand.Uint32()
	pk1.Check1 = ci
	pk1.Check2 = ci

	// Set our key type
	pk1.Keytype = ssh.KeyAlgoED25519

	// Add the pubkey to the optionally-encrypted block
	pk, ok := key.Public().(ed25519.PublicKey)
	if !ok {
		//fmt.Fprintln(os.Stderr, "ed25519.PublicKey type assertion failed on an ed25519 public key. This should never ever happen.")
		return nil
	}
	pubKey := []byte(pk)
	pk1.Pub = pubKey

	// Add our private key
	pk1.Priv = key

	// Might be useful to put something in here at some point
	pk1.Comment = ""

	// Add some padding to match the encryption block size within PrivKeyBlock (without Pad field)
	// 8 doesn't match the documentation, but that's what ssh-keygen uses for unencrypted keys. *shrug*
	bs := 8
	blockLen := len(ssh.Marshal(pk1))
	padLen := (bs - (blockLen % bs)) % bs
	pk1.Pad = make([]byte, padLen)

	// Padding is a sequence of bytes like: 1, 2, 3...
	for i := 0; i < padLen; i++ {
		pk1.Pad[i] = byte(i + 1)
	}

	// Generate the pubkey prefix "\0\0\0\nssh-ed25519\0\0\0 "
	prefix := []byte{0x0, 0x0, 0x0, 0x0b}
	prefix = append(prefix, []byte(ssh.KeyAlgoED25519)...)
	prefix = append(prefix, []byte{0x0, 0x0, 0x0, 0x20}...)

	// Only going to support unencrypted keys for now
	w.CipherName = "none"
	w.KdfName = "none"
	w.KdfOpts = ""
	w.NumKeys = 1
	w.PubKey = append(prefix, pubKey...)
	w.PrivKeyBlock = ssh.Marshal(pk1)

	magic = append(magic, ssh.Marshal(w)...)

	return magic
}
