package kcerts

import (
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/enfabrica/enkit/lib/cache"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/mitchellh/go-homedir"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
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
	if a.PID == 0 {
		return nil
	}
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

type AgentCert struct {
	MD5        string
	Principals []string
	Ext        map[string]string
	ValidFor   time.Duration
}

// Principals returns a map where the keys are the CA's PKS and the certs identities are the values
func (a SSHAgent) Principals() ([]AgentCert, error) {
	conn, err := net.Dial("unix", a.Socket)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	keys, err := agent.NewClient(conn).List()
	if err != nil {
		return nil, err
	}
	var toReturn []AgentCert
	for _, key := range keys {
		p, err := ssh.ParsePublicKey(key.Marshal())
		if err != nil {
			continue
		}
		if cert, ok := p.(*ssh.Certificate); ok {
			toReturn = append(toReturn, AgentCert{
				MD5:        ssh.FingerprintLegacyMD5(cert.SignatureKey),
				Principals: cert.ValidPrincipals,
				Ext:        cert.Extensions,
				ValidFor:   time.Unix(int64(cert.ValidBefore), 0).Sub(time.Now()),
			})
		}
	}
	return toReturn, err
}

// AddCertificates loads an ssh certificate into the agent.
// privateKey must be a key type accepted by the golang.org/x/ssh/agent AddedKey struct.
// At time of writing, this can be: *rsa.PrivateKey, *dsa.PrivateKey, ed25519.PrivateKey or *ecdsa.PrivateKey.
// Note that ed25519.PrivateKey should be passed by value.
func (a SSHAgent) AddCertificates(privateKey PrivateKey, publicKey ssh.PublicKey) error {
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
	ttl := SSHCertRemainingTTL(cert)
	if ttl == InValidCertTimeDuration {
		return fmt.Errorf("certificate is already expired or invalid, not adding")
	}
	return agentClient.Add(agent.AddedKey{
		PrivateKey:   privateKey.Raw(),
		Certificate:  cert,
		LifetimeSecs: uint32(ttl.Seconds()),
	})
}

func (a SSHAgent) GetEnv() []string {
	env := []string{fmt.Sprintf("SSH_AUTH_SOCK=%s", a.Socket)}
	if a.PID != 0 {
		env = append(env, fmt.Sprintf("SSH_AGENT_PID=%d", a.PID))
	}
	return env
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
	// If the SSH agent was started locally, both SSH_AGENT_SOCK and
	// SSH_AGENT_PID will be set.  However, when using ssh-agent forwarding over
	// an SSH or CRD session, only SSH_AUTH_SOCK will be set.
	envSSHSock := os.Getenv("SSH_AUTH_SOCK")
	envSSHPID := os.Getenv("SSH_AGENT_PID")
	if envSSHSock == "" {
		return nil
	}
	pid := 0
	if envSSHPID != "" {
		var err error
		pid, err = strconv.Atoi(envSSHPID)
		if err != nil {
			return nil
		}
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
func SignPublicKey(p PrivateKey, certType uint32, principals []string, ttl time.Duration, pub ssh.PublicKey, mods ...CertMod) (*ssh.Certificate, error) {
	// OpenSSH controls what the key allows through extensions.
	// See https://github.com/openssh/openssh-portable/blob/master/PROTOCOL.certkeys
	extensions := map[string]string{}
	if certType == 1 {
		extensions = map[string]string{
			"permit-agent-forwarding": "",
			"permit-x11-forwarding":   "",
			"permit-port-forwarding":  "",
			"permit-pty":              "",
			"permit-user-rc":          "",
		}
	}

	from := time.Now()
	to := time.Now().Add(ttl)
	cert := &ssh.Certificate{
		CertType:        certType,
		Key:             pub,
		ValidAfter:      uint64(from.Unix()),
		ValidBefore:     uint64(to.Unix()),
		ValidPrincipals: principals,
		Permissions: ssh.Permissions{
			Extensions: extensions,
		},
	}
	for _, m := range mods {
		cert = m(cert)
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
