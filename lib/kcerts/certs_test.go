package kcerts_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/enfabrica/enkit/lib/kcerts"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/ssh"
	"net"
	"testing"
	"time"
)

func TestCertSuite(t *testing.T) {
	opts, err := kcerts.NewOptions(
		kcerts.WithCountries([]string{"US"}),
		kcerts.WithOrganizations([]string{"Test Corp"}),
		kcerts.WithValidUntil(time.Now().AddDate(3, 0, 0)),
		kcerts.WithNotValidBefore(time.Now().Add(-10*time.Minute)),
		kcerts.WithIpAddresses([]net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("0.0.0.0")}),
	)

	assert.Nil(t, err)
	rootCert, rootPem, rootPrivateKey, err := kcerts.GenerateNewCARoot(opts)
	assert.Nil(t, err)

	intermediateCert, intermediatePem, intermediatePrivateKey, err := kcerts.GenerateIntermediateCertificate(opts, rootCert, rootPrivateKey)
	assert.Nil(t, err)

	serverCert, _, _, err := kcerts.GenerateServerKey(opts, intermediateCert, intermediatePrivateKey)
	t.Run("verify intermediate", func(t *testing.T) {
		assert.Nil(t, verifyIntermediateChain(rootPem, intermediateCert))
	})
	t.Run("verify chain", func(t *testing.T) {
		assert.Nil(t, verifyFullChain(rootPem, intermediatePem, serverCert))
	})
	t.Run("verify rsa client output with intermediate chain", func(t *testing.T) {
		assert.Nil(t, verifyRSAEncryption(intermediatePrivateKey))
	})
	t.Run("verify rsa client output with root chain", func(t *testing.T) {
		assert.Nil(t, verifyRSAEncryption(rootPrivateKey))
	})

	t.Run("test rsa output generations", func(t *testing.T) {
		publicBytes, privateBytes := kcerts.GenerateSSHKeyPair(rootPrivateKey)

		publicBlock, _ := pem.Decode(publicBytes)
		assert.NotNil(t, publicBlock)
		assert.Nil(t, err)
		assert.Equal(t, publicBlock.Type, "PUBLIC KEY")

		privateBlock, _ := pem.Decode(privateBytes)
		assert.NotNil(t, privateBlock)
		assert.Nil(t, err)
		assert.Equal(t, privateBlock.Type, "RSA PRIVATE KEY")

		privateKey, err := x509.ParsePKCS1PrivateKey(privateBlock.Bytes)
		assert.NotNil(t, privateKey)
		assert.Nil(t, err)
		assert.Nil(t, verifyRSAEncryption(privateKey))
	})
}

func verifyIntermediateChain(root []byte, inter *x509.Certificate) error {
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(root)
	opts := x509.VerifyOptions{
		Roots: roots,
	}
	if _, err := inter.Verify(opts); err != nil {
		return err
	}
	return nil
}

func verifyFullChain(root, inter []byte, child *x509.Certificate) error {
	roots := x509.NewCertPool()
	inters := x509.NewCertPool()
	roots.AppendCertsFromPEM(root)
	inters.AppendCertsFromPEM(inter)
	opts := x509.VerifyOptions{
		Roots:         roots,
		Intermediates: inters,
	}
	if _, err := child.Verify(opts); err != nil {
		return err
	}
	return nil
}

func verifyRSAEncryption(key *rsa.PrivateKey) error {
	data := []byte("Hello, world!")
	signer, _ := ssh.NewSignerFromKey(key)
	sig, _ := signer.Sign(rand.Reader, data)

	// Extract the ssh.PublicKey from *rsa.PublicKey to verify the signature.
	pub, _ := ssh.NewPublicKey(&key.PublicKey)
	if err := pub.Verify(data, sig); err != nil {
		return errors.New(fmt.Sprintf("publicKey.Verify failed: %v", err))
	}
	// Modify the data and make sure we get a failure.
	data[0]++
	if err := pub.Verify(data, sig); err == nil {
		return errors.New("modifying the data should have resulted in a verification error")
	}

	return nil
}

func TestCertTTL(t *testing.T) {
	_, sourcePrivKey, err := kcerts.GenerateED25519()
	assert.Nil(t, err)
	toBeSigned, _, err := kcerts.GenerateED25519()
	assert.Nil(t, err)
	// code of your test
	principalList := []string{"foo", "bar", "baz"}
	cert, err := kcerts.SignPublicKey(sourcePrivKey, 1, principalList, 5*time.Hour, toBeSigned)
	certTotalTTL := kcerts.SSHCertTotalTTL(cert)
	certRemainingTTL := kcerts.SSHCertRemainingTTL(cert)

	assert.Equal(t, certTotalTTL, 5 * time.Hour)
	assert.Greater(t, int(certTotalTTL), 0)

	assert.Less(t, certRemainingTTL.Seconds(), certTotalTTL.Seconds())
	assert.Greater(t, int(certRemainingTTL), 0)

}