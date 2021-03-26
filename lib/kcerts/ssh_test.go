package kcerts_test

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/kcerts"
	"golang.org/x/crypto/ssh"
	"testing"
)

func TestAddSSHCAToClient(t *testing.T) {
	opts, err := kcerts.NewOptions()
	if err != nil {
		t.Error(err)
	}
	_, _, privateKey, err := kcerts.GenerateNewCARoot(opts)
	if err != nil {
		t.Fatal(err)
	}
	sshpub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	err = kcerts.AddSSHCAToClient(sshpub, []string{"*.localhost", "localhost"})
	if err != nil {
		fmt.Println(err.Error())
	}
}

func TestStartSSHAgent(t *testing.T) {
	err := kcerts.StartSSHAgent()
	if err != nil {
		fmt.Println(err)
	}
}
