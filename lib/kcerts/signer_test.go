package kcerts

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/ssh"
	"reflect"
	"testing"
	"time"
)

var tableTestTypes = []SSHKeyGenerator{GenerateED25519, GenerateRSA}

// TestSha256Signer_PublicKey tests all possible combinations of supported PrivateKey signing ssh.PublicKeys
// It will sign the following ssh certs with the custom algos by their providers
func TestSha256Signer_PublicKey(t *testing.T) {
	for _, sourceType := range tableTestTypes {
		for _, toSignType := range tableTestTypes {
			_, sourcePrivKey, err := sourceType()
			assert.Nil(t, err)
			toBeSigned, _, err := toSignType()
			assert.Nil(t, err)
			t.Run(fmt.Sprintf("Source:%v,Sign:%v", reflect.TypeOf(sourcePrivKey), reflect.TypeOf(toBeSigned)), func(t *testing.T) {
				// code of your test
				r, err := SignPublicKey(sourcePrivKey, 1, []string{}, 5*time.Hour, toBeSigned)
				assert.Nil(t, err)
				assert.NotNil(t, r)
				fmt.Println(r.Type())
			})
		}
	}
}

// TestSha256Signer_PublicKey tests all possible combinations of supported PrivateKey signing ssh.PublicKeys
// It will sign the following ssh certs with the custom algos by their providers
func TestPemEncodeKeys(t *testing.T) {
	for _, sourceType := range tableTestTypes {
		_, priv, err := sourceType()
		assert.Nil(t, err)
		pemBytes, err := priv.SSHPemEncode()
		assert.Nil(t, err)
		_, err = ssh.ParsePrivateKey(pemBytes)
		assert.Nilf(t, err, "failed demarshalling private key for type %s", reflect.TypeOf(priv))
	}
}
