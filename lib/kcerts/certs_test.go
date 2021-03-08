package kcerts_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"github.com/enfabrica/enkit/lib/kcerts"
	"golang.org/x/crypto/ssh"
	"log"
	"testing"
)

func TestCert(t *testing.T) {
	rootCert, rootPem, rootPrivateKey, err := kcerts.GenerateNewCARoot()
	if err != nil {
		t.Fatal(err.Error())
	}
	intermediateCert, intermediatePem, intermediatePrivateKey, err := kcerts.GenerateIntermediateCertificate(rootCert, rootPrivateKey)
	if err != nil {
		t.Fatal(err.Error())
	}

	serverCert, _, _, err := kcerts.GenerateServerKey(intermediateCert, intermediatePrivateKey)
	t.Run("verify intermediate", func(t *testing.T) {
		if err := verifyIntermediateChain(rootPem, intermediateCert); err != nil {
			t.Error("error verifying intermediate chain", err)
		}

	})
	t.Run("verify chain", func(t *testing.T) {
		if err := verifyFullChain(rootPem, intermediatePem, serverCert); err != nil {
			t.Error("error verifying full chain", err)
		}
	})
	t.Run("verify rsa client output with intermediate chain", func(t *testing.T) {
		if err := verifyRSAEncryption(intermediatePrivateKey); err != nil {
			t.Error(err.Error())
		}
	})
	t.Run("verify rsa client output with root chain", func(t *testing.T) {
		if err := verifyRSAEncryption(rootPrivateKey); err != nil {
			t.Error(err.Error())
		}
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


func verifyRSAEncryption(key *rsa.PrivateKey) error{
	data := []byte("Hello, world!")
	signer, _ := ssh.NewSignerFromKey(key)
	sig, _ := signer.Sign(rand.Reader, data)

	// extract the ssh.PublicKey from *rsa.PublicKey to verify the signature
	pub, _ := ssh.NewPublicKey(&key.PublicKey)
	if err := pub.Verify(data, sig); err != nil {
		log.Fatalf("publicKey.Verify failed: %v", err)
	}
	// modify the data and make sure we get a failure
	data[0]++
	if err := pub.Verify(data, sig); err == nil {
		return errors.New("modifying the data should have resulted in a verification error")
	}

	return nil
}
