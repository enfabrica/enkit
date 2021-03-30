package kcerts_test

import (
	"github.com/enfabrica/enkit/lib/kcerts"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"testing"
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
	err := kcerts.StartSSHAgent()
	assert.Nil(t, err)
}
