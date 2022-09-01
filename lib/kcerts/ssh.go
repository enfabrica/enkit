package kcerts

import (
	"crypto/rand"
	"errors"
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
	"github.com/enfabrica/enkit/lib/config/directory"
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
	sockR        = regexp.MustCompile("(?m)SSH_AUTH_SOCK=([^;\\n]*)")
	pidR         = regexp.MustCompile("(?m)SSH_AGENT_PID=([0-9]*)")
	GetConfigDir = directory.GetConfigDir // to enable mocking
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

func (a *SSHAgent) GetStandardSocketPath() (string, error) {
	path, err := GetConfigDir("enkit")
	if err != nil {
		return "", err
	}

	// Securely make sure path exists and is permission 0700
	if err := os.MkdirAll(path, 0700); err != nil && !os.IsExist(err) {
		return "", err
	}
	if err := os.Chmod(path, 0700); err != nil {
		return "", err
	}

	socket := filepath.Join(path, "agent")

	return socket, nil
}

func (a *SSHAgent) UseStandardPaths() error {
	socket, err := a.GetStandardSocketPath()
	if err != nil {
		return err
	}

	if a.Socket == socket {
		// no standardization needed.
		return nil
	}

	// Securely rename socket:
	_, err = os.Stat(socket)
	if err == nil {
		err := os.Remove(socket)
		if err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Chmod(a.Socket, 0700); err != nil {
		return err
	}
	if err := os.Symlink(a.Socket, socket); err != nil {
		return err
	}
	a.Socket = socket

	return nil
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

// FindSSHAgent will attempt to find a working ssh-agent socket, or will create
// a new ssh-agent if necessary.  If a valid socket is found that does not
// correspond with the desired standard location, a symlink is created.
//
// In detail:
//   1) tries the socket indicated by the SSH_AUTH_SOCK environment variable.
//   2) otherwise, tries the socket indicated by the cache.
//   3) otherwise, starts a new ssh-agent.
//   4) ensures the valid socket has the right path, otherwise does a symlink.
//
// The final ssh-agent socket returned by FindSSHAgent is always
// ~/.config/enkit/agent.
func FindSSHAgent(store cache.Store, logger logger.Logger) (*SSHAgent, error) {
	var err error
	agent := FindSSHAgentFromEnv()
	if agent != nil {
		if !agent.Valid() {
			logger.Warnf("%s from env isn't a valid ssh-agent socket.", agent.Socket)
			agent = nil
		}
	}

	if agent == nil {
		agent, err = FetchSSHAgentFromCache(store)
		if err != nil {
			logger.Warnf("%s", err)
			agent = nil
		}
		if agent != nil {
			if !agent.Valid() {
				logger.Warnf("%s from cache isn't a valid ssh-agent socket.", agent.Socket)
				agent = nil
			}
		}
	}

	if agent == nil {
		agent, err = CreateNewSSHAgent()
		if err != nil {
			logger.Warnf("%s", err)
			return nil, err
		}
		if agent != nil {
			if !agent.Valid() {
				logger.Warnf("Newly created socket %s isn't a valid ssh-agent socket.", agent.Socket)
				agent = nil
			}
		}
	}

	if agent == nil {
		return nil, fmt.Errorf("Failed to find or create ssh-agent socket.")
	}

	// If we have a valid agent, make sure the paths are right.
	err = agent.UseStandardPaths()
	if err != nil {
		return nil, err
	}

	logger.Infof("%s", WriteAgentToCache(store, agent))
	return agent, nil
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
	a := &SSHAgent{}

	socket, err := a.GetStandardSocketPath()
	if err != nil {
		return nil, err
	}

	os.Remove(socket) // ignore errors
	_, err = os.Stat(socket)
	if err == nil {
		return nil, fmt.Errorf("unable to delete existing socket: %s", socket)
	}

	// TODO(jonathan): is it necessary or safer to delete the socket first?
	cmd := exec.Command("ssh-agent", "-s", "-a", socket)
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
	a.Socket = rawSock
	a.PID = pid
	a.Close = func() {
		_ = a.Kill()
	}
	return a, nil
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
