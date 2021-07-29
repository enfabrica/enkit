package kcerts

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"golang.org/x/crypto/ssh"
	"math/big"
	"net"
	"time"
)

// GenerateNewCARoot returns the new certificate anchor for a chain. This should ideally only be called once as rotating
// this will invalidate all existing private certs. Unless, you add it and reload and x509.CertPool in a server.
func GenerateNewCARoot(opts *certOptions) (*x509.Certificate, []byte, *rsa.PrivateKey, error) {
	rootTemplate := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Country:      opts.Country,
			Organization: opts.Organization,
			CommonName:   "Root CA",
		},
		NotBefore:             opts.Before,
		NotAfter:              opts.After,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            2,
		IPAddresses:           opts.IPAddresses,
	}
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, nil, err
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, &rootTemplate, &rootTemplate, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, nil, nil, err
	}
	b := pem.Block{Type: "CERTIFICATE", Bytes: certBytes}
	certPEM := pem.EncodeToMemory(&b)

	return &rootTemplate, certPEM, privateKey, nil
}

// GenerateIntermediateCertificate will generate the DCA and intermediate chain. It is acceptable to publicly share this chain.
// requires to call GenerateNewCARoot beforehand. Reusing Opts is recommended.
func GenerateIntermediateCertificate(opts *certOptions, RootCa *x509.Certificate, RootCaPrivateKey *rsa.PrivateKey) (*x509.Certificate, []byte, *rsa.PrivateKey, error) {
	intermediatePrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, nil, err
	}
	intermediate := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Country:      opts.Country,
			Organization: opts.Organization,
			CommonName:   "DCA",
		},
		NotBefore:             opts.Before,
		NotAfter:              opts.After,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        false,
		MaxPathLen:            1,
		IPAddresses:           opts.IPAddresses,
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, &intermediate, RootCa, &intermediatePrivateKey.PublicKey, RootCaPrivateKey)
	if err != nil {
		return nil, nil, nil, err
	}

	cert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		return nil, nil, nil, err
	}

	b := pem.Block{Type: "CERTIFICATE", Bytes: certBytes}
	certPEM := pem.EncodeToMemory(&b)

	return cert, certPEM, intermediatePrivateKey, nil
}

// GenerateServerKey will generate the final tls cert, generally requires GenerateIntermediateCertificate and
// GenerateNewCARoot to be called beforehand. Reusing Opts is recommended.
func GenerateServerKey(opts *certOptions, intermediateCert *x509.Certificate, intermediatePrivateKey *rsa.PrivateKey) (*x509.Certificate, []byte, *rsa.PrivateKey, error) {
	serverPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, nil, err
	}

	serverTemplate := &x509.Certificate{
		SerialNumber:   big.NewInt(1),
		NotBefore:      opts.Before,
		NotAfter:       opts.After,
		KeyUsage:       x509.KeyUsageCRLSign,
		ExtKeyUsage:    []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:           false,
		MaxPathLenZero: true,
		IPAddresses:    opts.IPAddresses,
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, serverTemplate, intermediateCert, &serverPrivateKey.PublicKey, intermediatePrivateKey)
	if err != nil {
		return nil, nil, nil, err
	}

	cert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		return nil, nil, nil, err
	}

	b := pem.Block{Type: "CERTIFICATE", Bytes: certBytes}
	certPEM := pem.EncodeToMemory(&b)

	return cert, certPEM, serverPrivateKey, nil
}

// GenerateSSHKeyPair will generate a re-encoded private rsa key and public key from an existing *rsa.PrivateKey.
func GenerateSSHKeyPair(privateKey *rsa.PrivateKey) ([]byte, []byte) {
	privateBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}

	publicBlock := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: x509.MarshalPKCS1PublicKey(&privateKey.PublicKey),
	}
	return pem.EncodeToMemory(publicBlock), pem.EncodeToMemory(privateBlock)
}

type Modifier func(o *certOptions) error

type certOptions struct {
	Country      []string
	Organization []string
	Before       time.Time
	After        time.Time
	IPAddresses  []net.IP
}

func WithCountries(countries []string) Modifier {
	return func(o *certOptions) error {
		o.Country = countries
		return nil
	}
}

func WithOrganizations(orgs []string) Modifier {
	return func(o *certOptions) error {
		o.Organization = orgs
		return nil
	}
}

func WithIpAddresses(ips []net.IP) Modifier {
	return func(o *certOptions) error {
		o.IPAddresses = ips
		return nil
	}
}

func WithValidUntil(validUntil time.Time) Modifier {
	return func(o *certOptions) error {
		o.After = validUntil
		currTime := time.Now()
		if currTime.After(o.After) {
			return fmt.Errorf("time %v to be valid until is less than current time %v", validUntil, currTime)
		}
		if o.After.Sub(currTime).Hours() < 24*365 { // hours in a year
			return fmt.Errorf("date configured with After of %v is too low, must be > one year", o.After)
		}
		return nil
	}
}

func WithNotValidBefore(startTime time.Time) Modifier {
	return func(o *certOptions) error {
		o.Before = startTime
		currTime := time.Now()
		if currTime.Before(o.Before) {
			return fmt.Errorf("time is invalid: value %v must be after current time %v", o.Before, currTime)
		}
		return nil
	}
}

func NewOptions(mods ...Modifier) (*certOptions, error) {
	co := &certOptions{}
	for _, mod := range mods {
		err := mod(co)
		if err != nil {
			return nil, err
		}
	}
	return co, nil
}

// SSHCertTotalTTL returns the total ttl of a cert.
func SSHCertTotalTTL(cert *ssh.Certificate) time.Duration {
	return time.Unix(int64(cert.ValidBefore), 0).Sub(time.Unix(int64(cert.ValidAfter), 0))
}
// SSHCertRemainingTTL returns the remaining ttl of a cert  when compared to current time.
func SSHCertRemainingTTL(cert *ssh.Certificate) time.Duration {
	return time.Unix(int64(cert.ValidBefore), 0).Sub(time.Now())
}

