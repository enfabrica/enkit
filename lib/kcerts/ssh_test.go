package kcerts_test

import (
	"github.com/enfabrica/enkit/lib/cache"
	"github.com/enfabrica/enkit/lib/kcerts"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"os"
	"testing"
	"time"
)

// TODO(adam): improve this test, including files writes and other edges cases
func TestAddSSHCAToClient(t *testing.T) {
	opts, err := kcerts.NewOptions()
	assert.Nil(t, err)
	_, _, privateKey, err := kcerts.GenerateNewCARoot(opts)
	assert.Nil(t, err)
	sshpub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	assert.Nil(t, err)
	_, err = kcerts.FindSSHDir()
	assert.Nil(t, err)
	tmpHome, err := ioutil.TempDir("", "en")
	assert.Nil(t, err)
	err = kcerts.AddSSHCAToClient(sshpub, []string{"*.localhost", "localhost"}, tmpHome)
	assert.Nil(t, err)
}

func TestStartSSHAgent(t *testing.T) {
	assert.Nil(t, os.Unsetenv("SSH_AUTH_SOCK"))
	assert.Nil(t, os.Unsetenv("SSH_AGENT_PID"))

	tmpDir, err := ioutil.TempDir("", "en")
	assert.Nil(t, err)
	localCache := &cache.Local{
		Root: tmpDir,
	}
	socketPath, pid, err := kcerts.FindSSHAgent(localCache, 5*time.Second)
	assert.Nil(t, err)
	assert.NotEqual(t, "", socketPath)
	assert.NotEqual(t, 0, pid)

	newSocketPath, newPID, err := kcerts.FindSSHAgent(localCache, 5*time.Second)
	assert.Nil(t, err)
	assert.Equal(t, socketPath, newSocketPath)
	assert.Equal(t, pid, newPID)

	time.Sleep(6 * time.Second)
	// Testing cache expiration
	newSocketPath, newPID, err = kcerts.FindSSHAgent(localCache, 5*time.Second)
	assert.Nil(t, err)
	assert.NotEqual(t, socketPath, newSocketPath)
	assert.NotEqual(t, pid, newPID)

}
