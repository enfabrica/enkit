package kcerts_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"github.com/enfabrica/enkit/lib/kcerts"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestSha256Signer_PublicKey(t *testing.T) {
	_, privKey, err := ed25519.GenerateKey(rand.Reader)
	assert.Nil(t, err)
	_, toBeSigned, err := kcerts.MakeKeys()
	assert.Nil(t, err)
	r, err := kcerts.SignPublicKey(privKey, 1, []string{}, 5 * time.Hour, toBeSigned)
	assert.Nil(t, err)
	fmt.Println("public thing is", r )
}
