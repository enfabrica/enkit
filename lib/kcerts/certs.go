package kcerts

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"time"
)

// GenerateNewCARoot returns the new certificate anchor for a chain. This should ideally only be called once as rotating
// this will invalidate all existing private certs. Unless, you add it and reload and x509.CertPool in a server.
func GenerateNewCARoot(opts *CertOptions) (*x509.Certificate, []byte, *rsa.PrivateKey, error) {
	if err := opts.Validate(); err != nil {
		return nil, nil, nil, errors.New("must call Validate() on certificate options before creating a CA")
	}
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
func GenerateIntermediateCertificate(opts *CertOptions, RootCa *x509.Certificate, RootCaPrivateKey *rsa.PrivateKey) (*x509.Certificate, []byte, *rsa.PrivateKey, error) {
	intermediatePrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := opts.Validate(); err != nil {
		return nil, nil, nil, errors.New("must call Validate() on certificate options before creating a DCA")
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
func GenerateServerKey(opts *CertOptions, intermediateCert *x509.Certificate, intermediatePrivateKey *rsa.PrivateKey) (*x509.Certificate, []byte, *rsa.PrivateKey, error) {
	serverPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := opts.Validate(); err != nil {
		return nil, nil, nil, errors.New("must call Validate() on certificate options before creating a Server Crt")
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

type CertOptions struct {
	Country      []string
	Organization []string
	Before       time.Time
	After        time.Time
	IPAddresses  []net.IP
}

func (o *CertOptions) WithCountries(cs []string) *CertOptions {
	o.Country = cs
	return o
}

func (o *CertOptions) WithOrganizations(orgs []string) *CertOptions {
	o.Organization = orgs
	return o
}

func (o *CertOptions) WithIpAddresses(ips []net.IP) *CertOptions {
	o.IPAddresses = ips
	return o
}

func (o *CertOptions) ValidUntil(validUntil time.Time) *CertOptions {
	o.After = validUntil
	return o
}
func (o *CertOptions) NotValidBefore(startTime time.Time) *CertOptions {
	o.Before = startTime
	return o
}

func (o *CertOptions) Validate() error {
	currTime := time.Now()
	if currTime.Before(o.Before) {
		return errors.New("time must be before time.Now")
	}
	if currTime.After(o.After) {
		return errors.New("cannot issue invalid CA's time invalid")
	}
	if o.After.Sub(currTime).Hours() < 24*365 { // hours in a year
		return errors.New("duration of the CA is too low")
	}
	if len(o.Organization) == 0 {
		return errors.New("must set organization")
	}
	if len(o.Country) == 0 {
		return errors.New("must set countries of origin for en_Lang and i8n support")
	}
	return nil
}

func NewOptions() *CertOptions {
	return &CertOptions{}
}
