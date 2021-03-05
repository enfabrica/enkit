package kcerts_test

import (
	"crypto/x509"
	"fmt"
	"github.com/enfabrica/enkit/lib/kcerts"
	"testing"
)

func TestCert(t *testing.T) {
	rootCert, _, rootPrivateKey, err := kcerts.GenerateNewCARoot()
	if err != nil {
		t.Fatal(err.Error())
	}
	intermediateCert, _, intermediatePrivateKey, err := kcerts.GenerateIntermediateCertificate(rootCert, rootPrivateKey)
	if err != nil {
		t.Fatal(err.Error())
	}

	serverCert, _, _, err := kcerts.GenerateServerKey(intermediateCert, intermediatePrivateKey)
	t.Run("verify intermediate", func(t *testing.T) {
		if err := verifyDCA(rootCert, intermediateCert); err != nil {
			t.Error("error verifying dca", err)
		}

	})
	t.Run("verify chain", func(t *testing.T) {
		if err := verifyLow(rootCert, intermediateCert, serverCert); err != nil {
			t.Error("error verifying dca", err)
		}
	})
}

//taken from https://gist.github.com/Mattemagikern/328cdd650be33bc33105e26db88e487d, which also helped with intermediate cert
//verification
func verifyDCA(root, dca *x509.Certificate) error {
	roots, err := x509.SystemCertPool()
	if err != nil {
		return err
	}
	roots.AddCert(root)
	opts := x509.VerifyOptions{
		Roots: roots,
	}

	if _, err := dca.Verify(opts); err != nil {
		return err
	}
	return nil
}

func verifyLow(root, DCA, child *x509.Certificate) error {
	roots := x509.NewCertPool()
	inter := x509.NewCertPool()
	roots.AddCert(root)
	inter.AddCert(DCA)
	opts := x509.VerifyOptions{
		Roots:         roots,
		Intermediates: inter,
	}
	if _, err := child.Verify(opts); err != nil {
		return err
	}
	fmt.Println("Low Verified")
	return nil
}
