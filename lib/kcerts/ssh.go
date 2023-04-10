package kcerts

import (
	"crypto/rand"
	"fmt"
	"io/ioutil"
	mathrand "math/rand"
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
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/multierror"
	"github.com/enfabrica/enkit/lib/srand"
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

	// Close will free the resources allocated by this SSHAgent object.
	//
	// If an ssh-agent was started, the Close() call will kill it.
	// If an ssh-agent was found in the environment, it will leave it running.
	Close func() `json:"-"`

	// How long to wait when connecting/reading/writing into the unix domain socket.
	Timeout time.Duration `json:"-"`
}

type SSHAgentModifier func(s *SSHAgent) error

type SSHAgentModifiers []SSHAgentModifier

func (m SSHAgentModifiers) Apply(a *SSHAgent) error {
	for _, mod := range m {
		if err := mod(a); err != nil {
			return err
		}
	}
	return nil
}

type SSHAgentFlags struct {
	Timeout time.Duration
}

const kDefaultTimeout = 10 * time.Second

func SSHAgentDefaultFlags() *SSHAgentFlags {
	return &SSHAgentFlags{
		Timeout: kDefaultTimeout,
	}
}

func (f *SSHAgentFlags) Register(set kflags.FlagSet, prefix string) *SSHAgentFlags {
	set.DurationVar(&f.Timeout, prefix+"ssh-agent-timeout", f.Timeout,
		"How long to wait before considering an agent unusable - read, write, connect timeout")
	return f
}

func WithFlags(f *SSHAgentFlags) SSHAgentModifier {
	return func(a *SSHAgent) error {
		a.Timeout = f.Timeout
		return nil
	}
}

func NewSSHAgent(mods ...SSHAgentModifier) (*SSHAgent, error) {
	agent := &SSHAgent{Timeout: kDefaultTimeout}
	if err := SSHAgentModifiers(mods).Apply(agent); err != nil {
		return nil, err
	}

	return agent, nil
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
	conn, err := net.DialTimeout("unix", a.Socket, a.Timeout)
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

	// Securely make sure path exists and its permissions are 0700
	if err := os.MkdirAll(path, 0700); err != nil && !os.IsExist(err) {
		return "", err
	}

	socket := filepath.Join(path, "agent")
	return socket, nil
}

func (a *SSHAgent) UseStandardPaths() error {
	standard_socket_path, err := a.GetStandardSocketPath()
	if err != nil {
		return err
	}

	if a.Socket == standard_socket_path {
		// no standardization needed.
		return nil
	}

	// Create symlink with a random name
	path, err := GetConfigDir("enkit")
	if err != nil {
		return err
	}
	tempname := fmt.Sprintf("%s/enkit.tmp%016x", path, mathrand.New(srand.Source).Uint64())
	defer os.Remove(tempname)
	if err := os.Symlink(a.Socket, tempname); err != nil {
		return fmt.Errorf("UseStandardPaths symlink failed: %w", err)
	}
	// Rename symlink to the standard name
	if err := os.Rename(tempname, standard_socket_path); err != nil {
		return fmt.Errorf("UseStandardPaths rename failed: %w", err)
	}

	a.Socket = standard_socket_path
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
	conn, err := net.DialTimeout("unix", a.Socket, a.Timeout)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(a.Timeout))
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
	conn, err := net.DialTimeout("unix", a.Socket, a.Timeout)
	if err != nil {
		return err
	}
	defer conn.Close()
	cert, ok := publicKey.(*ssh.Certificate)
	if !ok {
		return fmt.Errorf("public key is not a valid ssh certificate")
	}
	ttl := SSHCertRemainingTTL(cert)
	if ttl == InValidCertTimeDuration {
		return fmt.Errorf("certificate is already expired or invalid, not adding")
	}

	conn.SetDeadline(time.Now().Add(a.Timeout))
	return agent.NewClient(conn).Add(agent.AddedKey{
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

// FindOrCreateSSHAgent Will start the ssh agent in the interactive terminal if it isn't present already as an environment variable
// It will pull, in order: from the env, from the cache, create new.
func FindOrCreateSSHAgent(store cache.Store, logger logger.Logger, mods ...SSHAgentModifier) (*SSHAgent, error) {
	var errs []error
	agent, err := FindSSHAgentFromEnv(mods...)
	if err != nil {
		logger.Infof("%s", err)
		errs = append(errs, err)
	}
	if err == nil && agent != nil && agent.Valid() {
		return agent, nil
	}

	agent, err = FetchSSHAgentFromCache(store, mods...)
	if err != nil {
		logger.Infof("%s", err)
		errs = append(errs, err)
	}
	if err == nil && agent != nil && agent.Valid() {
		return agent, nil
	}

	agent, err = CreateNewSSHAgent(mods...)
	if err != nil {
		errs = append(errs, err)
		return nil, fmt.Errorf("could not start (or find) ssh agent - %w", multierror.New(errs))
	}
	if agent != nil && agent.Valid() {
		return agent, nil
	}
	return nil, fmt.Errorf("started ssh agent is not functional - other methods failed %w", multierror.New(errs))
}

// PrepareSSHAgent ensures that we end up with a working ssh-agent,
// either by discovering an existing ssh-agent or creating a new one.
// It also ensures that we have an up-to-date symlink to that agent's
// socket in the standard location.
//
// The final ssh-agent socket returned by PrepareSSHAgent is always
// ~/.config/enkit/agent.
func PrepareSSHAgent(store cache.Store, logger logger.Logger, mods ...SSHAgentModifier) (*SSHAgent, error) {
	agent, err := FindOrCreateSSHAgent(store, logger, mods...)
	if err != nil {
		return nil, err
	}

	// If we have a valid agent, make sure the paths are right.
	if err := agent.UseStandardPaths(); err != nil {
		return nil, err
	}

	logger.Infof("%s", WriteAgentToCache(store, agent))
	return agent, nil
}

// FindSSHAgentFromEnv creates an SSHAgent object based on environment variables.
//
// This is useful to configure an already running ssh-agent, post `eval ssh-agent`.
func FindSSHAgentFromEnv(mods ...SSHAgentModifier) (*SSHAgent, error) {
	// If the SSH agent was started locally, both SSH_AGENT_SOCK and
	// SSH_AGENT_PID will be set.  However, when using ssh-agent forwarding over
	// an SSH or CRD session, only SSH_AUTH_SOCK will be set.
	envSSHSock := os.Getenv("SSH_AUTH_SOCK")
	envSSHPID := os.Getenv("SSH_AGENT_PID")
	if envSSHSock == "" {
		return nil, fmt.Errorf("no SSH_AUTH_SOCK environment variable found")
	}
	pid := 0
	if envSSHPID != "" {
		var err error
		pid, err = strconv.Atoi(envSSHPID)
		if err != nil {
			return nil, fmt.Errorf("invalid PID in SSH_AGENT_PID environment: %s - %w", envSSHPID, err)
		}
	}

	agent, err := NewSSHAgent(mods...)
	if err != nil {
		return nil, err
	}

	agent.PID = pid
	agent.Socket = envSSHSock
	return agent, nil
}

// CreateNewSSHAgent creates a new ssh agent.
//
// Its env variables have not been added to the shell.
// It does not maintain its own connection.
func CreateNewSSHAgent(mods ...SSHAgentModifier) (*SSHAgent, error) {
	a, err := NewSSHAgent(mods...)
	if err != nil {
		return nil, err
	}

	socket, err := a.GetStandardSocketPath()
	if err != nil {
		return nil, fmt.Errorf("could not create ssh agent socket path: %w", err)
	}

	os.Remove(socket) // ignore errors
	_, err = os.Stat(socket)
	if err == nil {
		return nil, fmt.Errorf("could not delete existing ssh agent socket: %s", socket)
	}

	cmd := exec.Command("ssh-agent", "-s")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("starting ssh agent failed with error %w", err)
	}
	resultSock := sockR.FindStringSubmatch(string(out))
	resultPID := pidR.FindStringSubmatch(string(out))
	if len(resultSock) != 2 || len(resultPID) != 2 {
		return nil, fmt.Errorf("ssh agent returned an invalid pid or socket - %v %v in %v", resultSock, resultPID, string(out))
	}
	// The second element is the raw value we want
	rawPID := resultPID[1]
	rawSock := resultSock[1]

	pid, err := strconv.Atoi(rawPID)
	if err != nil {
		return nil, fmt.Errorf("error processing ssh agent pid %v: %w", resultPID, err)
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
