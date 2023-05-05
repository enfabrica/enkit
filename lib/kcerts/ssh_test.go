package kcerts

import (
	"flag"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/enfabrica/enkit/lib/cache"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/logger/klog"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/ssh"
)

// TODO(adam): improve this test, including files writes and other edges cases
func TestAddSSHCAToClient(t *testing.T) {
	hdir, _ := os.UserHomeDir()
	os.Setenv("HOME", "/tmp")
	defer func() {
		os.Setenv("HOME", hdir)
	}()

	opts, err := NewOptions()
	assert.NoError(t, err)
	_, _, privateKey, err := GenerateNewCARoot(opts)
	assert.NoError(t, err)
	sshpub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	assert.NoError(t, err)
	_, err = FindSSHDir()
	assert.NoError(t, err)
	tmpHome, err := ioutil.TempDir("", "en")
	assert.NoError(t, err)
	err = AddSSHCAToClient(sshpub, []string{"*.localhost", "localhost"}, tmpHome)
	assert.NoError(t, err)
}

// TODO(adam): test cache failures and edge cases
func TestStartSSHAgent(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "en")
	assert.NoError(t, err)
	old := GetConfigDir
	defer func() { GetConfigDir = old }()
	GetConfigDir = func(app string, namespaces ...string) (string, error) {
		return tmpDir + "/.config/enkit", nil
	}

	assert.NoError(t, os.Unsetenv("SSH_AUTH_SOCK"))
	assert.NoError(t, os.Unsetenv("SSH_AGENT_PID"))

	localCache := &cache.Local{
		Root: tmpDir,
	}
	l, err := klog.New("test", klog.FromFlags(*klog.DefaultFlags()))
	assert.NoError(t, err)

	agent, err := PrepareSSHAgent(localCache, WithLogging(l))
	assert.NoError(t, err)
	assert.NotEqual(t, "", agent.State.Socket)
	assert.NotEqual(t, 0, agent.State.PID)
	assert.NoError(t, agent.Valid())

	newAgent, err := PrepareSSHAgent(localCache, WithLogging(l))
	assert.NoError(t, err)
	assert.Equal(t, agent.State.Socket, newAgent.State.Socket)
	assert.Equal(t, agent.State.PID, newAgent.State.PID)
	assert.NoError(t, newAgent.Valid())

	newAgent, err = PrepareSSHAgent(localCache, WithLogging(l))
	assert.NoError(t, err)
	assert.Equal(t, agent.State.Socket, newAgent.State.Socket)
	assert.Equal(t, agent.State.PID, newAgent.State.PID)
	assert.NoError(t, newAgent.Valid())

	assert.NoError(t, DeleteSSHCache(localCache))
	time.Sleep(50 * time.Millisecond)

	//// Testing cache expiration
	agentAfterCacheDelete, err := PrepareSSHAgent(localCache, WithLogging(l))
	assert.NoError(t, err)
	// no longer valid: assert.NotEqual(t, newAgent.State.Socket, agentAfterCacheDelete.State.Socket)
	assert.NotEqual(t, newAgent.State.PID, agentAfterCacheDelete.State.PID)
	assert.NoError(t, agentAfterCacheDelete.Valid())

}

func TestSSHAgent_Principals(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "en")
	assert.NoError(t, err)
	old := GetConfigDir
	defer func() { GetConfigDir = old }()
	GetConfigDir = func(app string, namespaces ...string) (string, error) {
		return tmpDir + "/.config/enkit", nil
	}

	sourcePubKey, sourcePrivKey, err := GenerateED25519()
	assert.NoError(t, err)
	toBeSigned, toBeSignedPrivateKey, err := GenerateED25519()
	assert.NoError(t, err)
	// code of your test
	principalList := []string{"foo", "bar", "foo@enfabrica.net", "foo@ext.enfabrica.net", "baz"}
	cert, err := SignPublicKey(sourcePrivKey, 1, principalList, 5*time.Hour, toBeSigned)
	localCache := &cache.Local{
		Root: tmpDir,
	}
	l, err := klog.New("test", klog.FromFlags(*klog.DefaultFlags()))
	assert.NoError(t, err)
	a, err := PrepareSSHAgent(localCache, WithLogging(l))
	assert.NoError(t, err)
	err = a.AddCertificates(toBeSignedPrivateKey, cert)
	assert.NoError(t, err)
	res, err := a.Principals()
	assert.NoError(t, err)
	for _, v := range res {
		assert.Equal(t, ssh.FingerprintLegacyMD5(sourcePubKey), v.MD5)
		assert.Equal(t, principalList, v.Principals)
	}
}

func TestSSHAgentTimeout(t *testing.T) {
	dir, err := os.MkdirTemp("", "test-uds-ssh")
	assert.NoError(t, err)

	// The code here creates a listening socket, exposes it as an agent would,
	// but.. has no code to process connections. An ssh-agent client would timeout.
	sockaddr := filepath.Join(dir, "test-socket")
	l, err := net.Listen("unix", sockaddr)
	assert.NoError(t, err)
	defer l.Close()
	os.Setenv("SSH_AUTH_SOCK", sockaddr)

	agent, err := NewSSHAgent(WithTimeout(1 * time.Second))
	assert.NoError(t, err)
	assert.NotNil(t, agent)

	// SSH agent in environment should time out.
	assert.NoError(t, agent.LoadFromEnvironment())
	assert.Equal(t, 0, agent.State.PID)
	assert.Equal(t, sockaddr, agent.State.Socket)
	assert.Error(t, agent.Valid())

	// FindOrCreateSSHAgent should detect the problem and fail.
	store := &cache.Local{Root: dir}
	GetConfigDir = func(sub string, namespaces ...string) (string, error) {
		return filepath.Join(dir, sub), nil
	}
	agent, err = FindOrCreateSSHAgent(store)
	assert.NoError(t, err)
	assert.NotNil(t, agent)
	defer agent.Close()

	assert.NotEqual(t, 0, agent.State.PID)
	assert.NotEqual(t, "", agent.State.Socket)
}

func TestSSHAgentFlags(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)

	flags := SSHAgentDefaultFlags()
	flags.Register(&kflags.GoFlagSet{fs}, "t-")

	os.Unsetenv("SSH_AUTH_SOCK")
	fakeAgent := "echo SSH_AUTH_SOCK=/tmp/agent-from-flags; echo SSH_AGENT_PID=9786"
	assert.NoError(t, fs.Parse([]string{
		"--t-ssh-agent-timeout=2s",
		"--t-ssh-agent-command=/bin/sh",
		"--t-ssh-agent-flags=-c",
		"--t-ssh-agent-flags=" + fakeAgent,
	}))

	// Create new agent straight from flags.
	agent, err := NewSSHAgent(WithFlags(flags))
	assert.NoError(t, err)
	assert.Equal(t, 2*time.Second, agent.timeout)
	assert.Equal(t, "/bin/sh", agent.agentPath)
	assert.Equal(t, []string{"-c", fakeAgent}, agent.agentArgs)
	assert.NoError(t, agent.CreateNew())
	assert.Equal(t, 9786, agent.State.PID)
	assert.Equal(t, "/tmp/agent-from-flags", agent.State.Socket)

	//  Test agent detection, flags should still be used.
	dir, err := os.MkdirTemp("", "test-uds-ssh")
	assert.NoError(t, err)
	store := &cache.Local{Root: dir}
	agent, err = FindOrCreateSSHAgent(store, WithFlags(flags))
	assert.ErrorContains(t, err, "environment - no SSH_AUTH_SOCK")
	assert.ErrorContains(t, err, "cache - ssh agent cached entry does not exist")
	// This proves that the agent from flags was attempted.
	assert.ErrorContains(t, err, "new - invalid agent - could not connect - dial unix /tmp/agent-from-flags")
}
