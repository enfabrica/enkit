package kcerts_test

import (
	"io/ioutil"
	"os"
	"os/user"
	"testing"
	"time"

	"github.com/enfabrica/enkit/lib/cache"
	"github.com/enfabrica/enkit/lib/kcerts"
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

	opts, err := kcerts.NewOptions()
	assert.NoError(t, err)
	_, _, privateKey, err := kcerts.GenerateNewCARoot(opts)
	assert.NoError(t, err)
	sshpub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	assert.NoError(t, err)
	_, err = kcerts.FindSSHDir()
	assert.NoError(t, err)
	tmpHome, err := ioutil.TempDir("", "en")
	assert.NoError(t, err)
	err = kcerts.AddSSHCAToClient(sshpub, []string{"*.localhost", "localhost"}, tmpHome)
	assert.NoError(t, err)
}

// TODO(adam): test cache failures and edge cases
func TestStartSSHAgent(t *testing.T) {
	old := kcerts.UserCurrent
	defer func() { kcerts.UserCurrent = old }()
	kcerts.UserCurrent = func() (*user.User, error) {
		u := &user.User{
			Username: "testuser",
		}
		return u, nil
	}
	assert.Nil(t, os.Unsetenv("SSH_AUTH_SOCK"))
	assert.Nil(t, os.Unsetenv("SSH_AGENT_PID"))

	tmpDir, err := ioutil.TempDir("", "en")
	assert.Nil(t, err)
	localCache := &cache.Local{
		Root: tmpDir,
	}
	l, err := klog.New("test", klog.FromFlags(*klog.DefaultFlags()))
	assert.Nil(t, err)

	agent, err := kcerts.FindSSHAgent(localCache, l)
	assert.Nil(t, err)
	assert.NotEqual(t, "", agent.Socket)
	assert.NotEqual(t, 0, agent.PID)
	assert.True(t, agent.Valid())

	newAgent, err := kcerts.FindSSHAgent(localCache, l)
	assert.Nil(t, err)
	assert.Equal(t, agent.Socket, newAgent.Socket)
	assert.Equal(t, agent.PID, newAgent.PID)
	assert.True(t, newAgent.Valid())

	newAgent, err = kcerts.FindSSHAgent(localCache, l)
	assert.Nil(t, err)
	assert.Equal(t, agent.Socket, newAgent.Socket)
	assert.Equal(t, agent.PID, newAgent.PID)
	assert.True(t, newAgent.Valid())

	assert.Nil(t, kcerts.DeleteSSHCache(localCache))
	time.Sleep(50 * time.Millisecond)

	//// Testing cache expiration
	agentAfterCacheDelete, err := kcerts.FindSSHAgent(localCache, l)
	assert.Nil(t, err)
	// no longer valid: assert.NotEqual(t, newAgent.Socket, agentAfterCacheDelete.Socket)
	assert.NotEqual(t, newAgent.PID, agentAfterCacheDelete.PID)
	assert.True(t, agentAfterCacheDelete.Valid())

}

func TestSSHAgent_Principals(t *testing.T) {
	sourcePubKey, sourcePrivKey, err := kcerts.GenerateED25519()
	assert.Nil(t, err)
	toBeSigned, toBeSignedPrivateKey, err := kcerts.GenerateED25519()
	assert.Nil(t, err)
	tmpDir, err := ioutil.TempDir("", "en")
	assert.Nil(t, err)
	// code of your test
	principalList := []string{"foo", "bar", "baz"}
	cert, err := kcerts.SignPublicKey(sourcePrivKey, 1, principalList, 5*time.Hour, toBeSigned)
	localCache := &cache.Local{
		Root: tmpDir,
	}
	l, err := klog.New("test", klog.FromFlags(*klog.DefaultFlags()))
	assert.Nil(t, err)
	a, err := kcerts.FindSSHAgent(localCache, l)
	assert.Nil(t, err)
	err = a.AddCertificates(toBeSignedPrivateKey, cert)
	assert.Nil(t, err)
	res, err := a.Principals()
	assert.Nil(t, err)
	for _, v := range res {
		assert.Equal(t, ssh.FingerprintLegacyMD5(sourcePubKey), v.MD5)
		assert.Equal(t, principalList, v.Principals)
	}
}
